package argocd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoServerApiClient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrClientClosed is returned by every RPC once Close has been called, so a
// request that outlives shutdown fails loudly instead of quietly dialing a new
// connection nobody will ever close.
var ErrClientClosed = errors.New("argocd client is closed")

// ArgoCDClient defines the interface for interacting with ArgoCD
type IArgoCDClient interface {
	// List returns all ArgoCD applications
	List(ctx context.Context, in *application.ApplicationQuery) (*v1alpha1.ApplicationList, error)
	GetApplicationManifests(ctx context.Context, in *application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error)
	Get(ctx context.Context, in *application.ApplicationQuery) (*v1alpha1.Application, error)
	GetUrl() string
	// GetScheme returns the URL scheme ("http" or "https") this client's ArgoCD instance is
	// reachable on, for building deep links back into its UI.
	GetScheme() string
	// Close releases this client's connection, which also stops the local
	// gRPC-Web proxy behind it. Safe to call more than once.
	Close() error
}

type ArgoCDClientOptions struct {
	Address         string
	Insecure        bool
	PlainText       bool
	AuthTokenEnvVar string

	// Name identifies this ArgoCD instance in logs and metrics. Callers pass the
	// key the instance is configured under.
	Name string
	// Logger defaults to slog.Default() when nil. It's injected rather than taken
	// from internal/tangle, which already imports this package.
	Logger *slog.Logger
}

// connection bundles an ArgoCD application client with the closer that releases
// it, so the two can never drift apart. The closer tears down both the gRPC
// ClientConn and the local gRPC-Web proxy (unix socket, grpc.Server, goroutine)
// that argo-cd's apiclient starts underneath it.
type connection struct {
	applications application.ApplicationServiceClient
	closer       io.Closer
	// generation counts from 1 and increases with every successful dial, so a
	// caller holding a connection can tell whether it's still the current one.
	generation uint64
}

type ArgoCDClient struct {
	Options *ArgoCDClientOptions

	log       *slog.Logger
	authToken string
	// dial is a field so tests can exercise the reconnect machinery without a
	// live ArgoCD. Production always uses dialArgoCD.
	dial func() (application.ApplicationServiceClient, io.Closer, error)
	// backoff is how long to wait before the given transient retry (1-based).
	// A field so tests don't sleep. Production always uses transientBackoff.
	backoff func(attempt int) time.Duration

	mu         sync.Mutex
	conn       *connection
	generation uint64
	closed     bool
}

func NewArgoCDClient(options *ArgoCDClientOptions) (IArgoCDClient, error) {
	authToken, found := os.LookupEnv(options.AuthTokenEnvVar)
	if !found {
		return nil, fmt.Errorf("auth token not found")
	}

	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	client := &ArgoCDClient{
		Options:   options,
		log:       logger.With(slog.String("argocd", options.Name)),
		authToken: authToken,
	}
	client.dial = client.dialArgoCD
	client.backoff = transientBackoff

	// Connect eagerly so a broken instance shows up in the logs at boot rather
	// than on someone's first request — but don't treat failure as fatal. With
	// GRPCWeb this dial only proves the local proxy came up (it never contacts
	// ArgoCD), and anything that does go wrong is recovered by the lazy connect
	// on first use.
	if _, err := client.connection(); err != nil {
		client.log.Warn("Could not connect to ArgoCD at startup, will retry on first use.", slog.Any("error", err))
	}

	return client, nil
}

// dialArgoCD builds a new connection to ArgoCD. Every call starts its own
// gRPC-Web proxy, so the caller owns the returned closer.
func (c *ArgoCDClient) dialArgoCD() (application.ApplicationServiceClient, io.Closer, error) {
	argocdClient, err := apiclient.NewClient(&apiclient.ClientOptions{
		ServerAddr: c.Options.Address,
		Insecure:   c.Options.Insecure,
		PlainText:  c.Options.PlainText,
		AuthToken:  c.authToken,
		GRPCWeb:    true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("creating ArgoCD API client: %w", err)
	}

	closer, applicationsClient, err := argocdClient.NewApplicationClient()
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to ArgoCD: %w", err)
	}

	return applicationsClient, closer, nil
}

// connection returns the current connection, dialing one if this client doesn't
// have it yet.
func (c *ArgoCDClient) connection() (*connection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	if c.conn != nil {
		return c.conn, nil
	}

	return c.connectLocked(dialReasonInitial)
}

// connectLocked dials and installs a new connection. Callers hold c.mu.
func (c *ArgoCDClient) connectLocked(reason string) (*connection, error) {
	applications, closer, err := c.dial()
	if err != nil {
		clientDialsTotal.WithLabelValues(c.Options.Name, reason, dialResultFailure).Inc()
		return nil, err
	}

	c.generation++
	c.conn = &connection{
		applications: applications,
		closer:       closer,
		generation:   c.generation,
	}

	clientDialsTotal.WithLabelValues(c.Options.Name, reason, dialResultSuccess).Inc()
	clientConnectionGeneration.WithLabelValues(c.Options.Name).Set(float64(c.generation))

	return c.conn, nil
}

// reconnect replaces stale with a freshly dialed connection. If another caller
// got there first it returns that one instead of dialing again: a dead transport
// fails every in-flight RPC at once, and the pond pools in ArgoCDWrapper mean
// there can be a dozen of them, which should cost one dial between them.
//
// That collapsing only applies once a replacement dial has succeeded. A failed
// one leaves no current connection, so the callers behind it each dial for
// themselves. That's deliberate: being locked out of recovery because one redial
// failed would be worse than a few redundant dials, and bounding them is the
// rate limit deferred in docs/adrs/0025-reconnect-the-argocd-grpc-client.md.
func (c *ArgoCDClient) reconnect(stale *connection) (*connection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	if c.conn != nil && c.conn != stale {
		return c.conn, nil
	}

	if c.conn != nil {
		// Closing aborts anything still in flight on this connection. Those
		// callers see codes.Canceled, which retries here on the new generation.
		if err := c.conn.closer.Close(); err != nil {
			c.log.Debug("Error closing the previous ArgoCD connection.", slog.Any("error", err))
		}
		c.conn = nil
	}

	return c.connectLocked(dialReasonReconnect)
}

// maxTransientRetries bounds how many times one call is repeated after a
// transient transport failure. A truncated response is rare and uncorrelated
// (about 1 in 1,000 through a busy ingress in testing), so a second failure in
// a row almost always means something is actually wrong.
const maxTransientRetries = 2

// transientBackoff waits a little longer before each retry, with jitter so a
// burst of failures from one fan-out doesn't come back in lockstep.
func transientBackoff(attempt int) time.Duration {
	base := time.Duration(attempt) * 100 * time.Millisecond
	return base/2 + rand.N(base/2+1)
}

// callWithReconnect runs op against the current connection and recovers from
// two kinds of failure that aren't ArgoCD's answer to the request:
//
//   - The connection itself is gone (isConnectionLost). The connection argo-cd's
//     SDK hands us can never re-establish itself: util/grpc.BlockingNewClient
//     dials once and installs a "dialer" closed over that single net.Conn, so
//     when gRPC re-creates the transport — after its 30-minute idle timeout, a
//     GOAWAY, or any transport error — every subsequent RPC fails for the life
//     of the process. This redials once and runs op again. See
//     docs/adrs/0025-reconnect-the-argocd-grpc-client.md.
//   - The response was lost between ArgoCD and us
//     (isTransientTransportFailure), typically a reverse proxy cutting a
//     gRPC-Web response off partway through. The connection is fine, so this
//     waits briefly and runs op again, up to maxTransientRetries times. See
//     docs/adrs/0028-retry-transient-argocd-transport-failures.md.
//
// Every RPC this package makes is a read, so repeating one is safe. The one
// with a side effect, GetManifests' preceding hard-refresh Get, is repeated
// knowingly: a second refresh supersedes the first (ADR 0025 covers the cost).
//
// This is a generic function rather than a method because Go methods can't take
// type parameters.
func callWithReconnect[T any](ctx context.Context, c *ArgoCDClient, method string, op func(application.ApplicationServiceClient) (T, error)) (T, error) {
	var zero T

	conn, err := c.connection()
	if err != nil {
		return zero, err
	}

	reconnected := false
	transientRetries := 0
	for {
		result, err := op(conn.applications)

		switch {
		case err == nil:
			if reconnected || transientRetries > 0 {
				c.log.Info("ArgoCD call succeeded after retrying.",
					slog.String("method", method),
					slog.Bool("reconnected", reconnected),
					slog.Int("transient_retries", transientRetries),
					slog.Uint64("generation", conn.generation))
			}
			return result, nil

		case !reconnected && isConnectionLost(ctx, err):
			c.log.Warn("Lost the connection to ArgoCD, reconnecting.",
				slog.String("method", method),
				slog.Uint64("generation", conn.generation),
				slog.Any("error", err))

			fresh, reconnectErr := c.reconnect(conn)
			if reconnectErr != nil {
				c.log.Error("Could not reconnect to ArgoCD.",
					slog.String("method", method),
					slog.Any("error", reconnectErr))
				return zero, errors.Join(err, reconnectErr)
			}
			clientRetriesTotal.WithLabelValues(c.Options.Name, method, retryReasonReconnect).Inc()
			conn = fresh
			reconnected = true

		case transientRetries < maxTransientRetries && isTransientTransportFailure(ctx, err):
			transientRetries++
			wait := c.backoff(transientRetries)
			c.log.Warn("ArgoCD response was lost in transit, retrying.",
				slog.String("method", method),
				slog.Int("attempt", transientRetries),
				slog.Duration("backoff", wait),
				slog.Any("error", err))

			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return zero, errors.Join(err, ctx.Err())
			case <-timer.C:
			}
			clientRetriesTotal.WithLabelValues(c.Options.Name, method, retryReasonTransient).Inc()

		default:
			return result, err
		}
	}
}

// transientTransportMessages are the ways a response lost in transit shows up.
// argo-cd's gRPC-Web proxy returns these as codes.Unknown with the Go error's
// text as the message, so the text is all there is to go on:
//
//   - "unexpected EOF": the response body ended before its gRPC-Web trailer
//     frame. pkg/apiclient/grpcproxy.go turns a short read into
//     io.ErrUnexpectedEOF. This is what an ingress closing a chunked
//     HTTP/1.1 response partway through produces.
//   - ": EOF" and "connection reset by peer": the request's connection was
//     closed before any response arrived, which net/http reports from
//     http.Client.Do as `Post "<url>": EOF` or a reset read.
var transientTransportMessages = []string{
	io.ErrUnexpectedEOF.Error(),
	": EOF",
	"connection reset by peer",
}

// isTransientTransportFailure reports whether err means the response was lost
// between ArgoCD and this process, rather than being ArgoCD's answer.
//
// ADR 0025 chose status codes over error text for isConnectionLost. That
// isn't possible here: these failures arrive as codes.Unknown, the same code
// as a genuine error from ArgoCD, so the message is the only signal. Matching
// is anchored to the exact texts above so an ArgoCD error that happens to
// mention them isn't retried.
func isTransientTransportFailure(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}

	s, ok := status.FromError(err)
	if !ok || s.Code() != codes.Unknown {
		return false
	}

	message := s.Message()
	for _, transient := range transientTransportMessages {
		if message == transient || strings.HasSuffix(message, transient) {
			return true
		}
	}

	return false
}

// isConnectionLost reports whether err means this client's connection is gone
// and a new one might work.
//
// codes.Unavailable is the idle-timeout case: gRPC tried to re-create the
// transport and failed. codes.Canceled with a live context is grpc-go's
// ErrClientConnClosing — an RPC that was in flight when the connection was
// closed underneath it, which is what a sibling goroutine sees while another
// one reconnects. A caller-cancelled context produces the same code, hence the
// ctx.Err() guard.
//
// Neither code is exclusively a local signal: argo-cd's gRPC-Web proxy forwards
// ArgoCD's own Grpc-Status header verbatim, so a server-side Unavailable reaches
// us looking the same and costs one wasted redial. That's accepted — see the
// ADR. The common upstream failures (ArgoCD unreachable, a non-200 response)
// arrive as codes.Unknown and don't land here.
func isConnectionLost(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}

	switch status.Code(err) {
	case codes.Unavailable, codes.Canceled:
		return true
	default:
		return false
	}
}

func (c *ArgoCDClient) List(ctx context.Context, query *application.ApplicationQuery) (*v1alpha1.ApplicationList, error) {
	return callWithReconnect(ctx, c, "List", func(client application.ApplicationServiceClient) (*v1alpha1.ApplicationList, error) {
		return client.List(ctx, query)
	})
}

func (c *ArgoCDClient) GetApplicationManifests(ctx context.Context, query *application.ApplicationManifestQuery) (*repoServerApiClient.ManifestResponse, error) {
	return callWithReconnect(ctx, c, "GetManifests", func(client application.ApplicationServiceClient) (*repoServerApiClient.ManifestResponse, error) {
		return client.GetManifests(ctx, query)
	})
}

func (c *ArgoCDClient) Get(ctx context.Context, query *application.ApplicationQuery) (*v1alpha1.Application, error) {
	return callWithReconnect(ctx, c, "Get", func(client application.ApplicationServiceClient) (*v1alpha1.Application, error) {
		return client.Get(ctx, query)
	})
}

func (c *ArgoCDClient) GetUrl() string {
	return c.Options.Address
}

func (c *ArgoCDClient) GetScheme() string {
	if c.Options.PlainText {
		return "http"
	}

	return "https"
}

func (c *ArgoCDClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.conn == nil {
		return nil
	}

	err := c.conn.closer.Close()
	c.conn = nil

	return err
}
