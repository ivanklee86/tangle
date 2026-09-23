package argocd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoServerApiClient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestNewArgoCDClient/creates client with invalid options is the one
// TestNewArgoCDClient case that doesn't need a live ArgoCD: ARGOCD_TOKEN1 is
// a deliberately-nonexistent env var name, so NewArgoCDClient's
// os.LookupEnv check fails before any network call is attempted — no
// .env/t.Setenv needed, since nothing in this environment ever sets that
// name. The "valid options" case moved to client_e2e_test.go: unlike this
// one, it needs a real ArgoCD to dial successfully (see that file's comment).
func TestNewArgoCDClient(t *testing.T) {
	tests := []struct {
		name    string
		options *ArgoCDClientOptions
		wantErr bool
	}{
		{
			name: "creates client with invalid options",
			options: &ArgoCDClientOptions{
				Address:         "https://localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewArgoCDClient(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

// The tests below cover ArgoCDClient's connection handling: the promise is that
// a caller keeps getting answers after the connection underneath the client
// dies, which it does routinely (argo-cd's SDK builds a connection that can
// never re-dial itself — see docs/adrs/0025-reconnect-the-argocd-grpc-client.md).
// They drive it through the injectable dial func, so no ArgoCD is involved;
// client_e2e_test.go exercises the same machinery over the real wire.

// stubApplications implements the three ApplicationServiceClient methods this
// package calls, returning a scripted sequence of errors. The embedded
// interface is nil on purpose: any other method panics rather than quietly
// returning a zero value.
type stubApplications struct {
	application.ApplicationServiceClient

	mu sync.Mutex
	// errs is consumed one entry per call; the last entry repeats forever.
	errs  []error
	calls int
}

func (s *stubApplications) next() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++
	if s.calls <= len(s.errs) {
		return s.errs[s.calls-1]
	}

	return s.errs[len(s.errs)-1]
}

func (s *stubApplications) List(_ context.Context, _ *application.ApplicationQuery, _ ...grpc.CallOption) (*v1alpha1.ApplicationList, error) {
	if err := s.next(); err != nil {
		return nil, err
	}

	return &v1alpha1.ApplicationList{}, nil
}

func (s *stubApplications) Get(_ context.Context, _ *application.ApplicationQuery, _ ...grpc.CallOption) (*v1alpha1.Application, error) {
	if err := s.next(); err != nil {
		return nil, err
	}

	return &v1alpha1.Application{}, nil
}

func (s *stubApplications) GetManifests(_ context.Context, _ *application.ApplicationManifestQuery, _ ...grpc.CallOption) (*repoServerApiClient.ManifestResponse, error) {
	if err := s.next(); err != nil {
		return nil, err
	}

	return &repoServerApiClient.ManifestResponse{}, nil
}

// countingCloser stands in for the closer argo-cd's apiclient returns, which in
// production also stops the local gRPC-Web proxy.
type countingCloser struct {
	mu     sync.Mutex
	closes int
}

func (c *countingCloser) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++

	return nil
}

func (c *countingCloser) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.closes
}

// scriptedConn is one entry in a dialScript: either a connection to hand out,
// or an error to fail the dial with.
type scriptedConn struct {
	apps   *stubApplications
	closer *countingCloser
	err    error
}

func okConn(errs ...error) *scriptedConn {
	return &scriptedConn{apps: &stubApplications{errs: errs}, closer: &countingCloser{}}
}

func failedDial(err error) *scriptedConn {
	return &scriptedConn{err: err}
}

// dialScript hands out one connection per dial, in order. A dial past the end
// of the script fails loudly, so a runaway reconnect loop shows up as a test
// failure instead of passing quietly.
type dialScript struct {
	mu    sync.Mutex
	conns []*scriptedConn
	dials int
}

func (d *dialScript) dial() (application.ApplicationServiceClient, io.Closer, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.dials++
	if d.dials > len(d.conns) {
		return nil, nil, fmt.Errorf("unscripted dial #%d (script has %d)", d.dials, len(d.conns))
	}

	conn := d.conns[d.dials-1]
	if conn.err != nil {
		return nil, nil, conn.err
	}

	return conn.apps, conn.closer, nil
}

func (d *dialScript) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.dials
}

func newTestClient(t *testing.T, conns ...*scriptedConn) (*ArgoCDClient, *dialScript) {
	t.Helper()

	script := &dialScript{conns: conns}
	client := &ArgoCDClient{
		// A per-test name keeps the process-wide metric vectors from sharing
		// label sets between tests.
		Options: &ArgoCDClientOptions{Name: t.Name()},
		log:     slog.New(slog.DiscardHandler),
		dial:    script.dial,
		backoff: func(int) time.Duration { return 0 },
	}

	return client, script
}

func unavailable() error {
	return status.Error(codes.Unavailable, `transport: failed to write client preface: use of closed network connection`)
}

func TestArgoCDClient_ServesWithoutReconnecting(t *testing.T) {
	conn := okConn(nil)
	client, script := newTestClient(t, conn)

	got, err := client.List(context.Background(), &application.ApplicationQuery{})

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, 1, script.count(), "a healthy call should dial once and stay on that connection")
	assert.Equal(t, 0, conn.closer.count())
}

// The reported bug: the connection dies, and every later call fails forever.
// Each of the three RPCs has to recover on its own, so all three are covered.
func TestArgoCDClient_ReconnectsWhenTheConnectionIsLost(t *testing.T) {
	calls := map[string]func(*ArgoCDClient) (any, error){
		"List": func(c *ArgoCDClient) (any, error) {
			return c.List(context.Background(), &application.ApplicationQuery{})
		},
		"Get": func(c *ArgoCDClient) (any, error) {
			return c.Get(context.Background(), &application.ApplicationQuery{})
		},
		"GetApplicationManifests": func(c *ArgoCDClient) (any, error) {
			return c.GetApplicationManifests(context.Background(), &application.ApplicationManifestQuery{})
		},
	}

	// codes.Canceled is grpc-go's ErrClientConnClosing: what an RPC that was in
	// flight sees when the connection is closed underneath it, which is how a
	// sibling goroutine experiences another goroutine's reconnect.
	for _, lost := range []error{unavailable(), status.Error(codes.Canceled, "grpc: the client connection is closing")} {
		for name, call := range calls {
			t.Run(fmt.Sprintf("%s/%s", name, status.Code(lost)), func(t *testing.T) {
				dead, alive := okConn(lost), okConn(nil)
				client, script := newTestClient(t, dead, alive)

				got, err := call(client)

				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, 2, script.count(), "should have redialed exactly once")
				assert.Equal(t, 1, dead.closer.count(), "the dead connection should be released, not leaked")
			})
		}
	}
}

func TestArgoCDClient_RetriesOnlyOnce(t *testing.T) {
	client, script := newTestClient(t, okConn(unavailable()), okConn(unavailable()))

	_, err := client.List(context.Background(), &application.ApplicationQuery{})

	assert.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.Equal(t, 2, script.count(), "a still-failing retry must not loop")
}

// An error from ArgoCD itself says nothing about our connection, so reconnecting
// would just be a wasted dial and a second hard refresh.
func TestArgoCDClient_DoesNotReconnectOnApplicationErrors(t *testing.T) {
	for _, code := range []codes.Code{codes.NotFound, codes.PermissionDenied, codes.Unknown} {
		t.Run(code.String(), func(t *testing.T) {
			conn := okConn(status.Error(code, "from argocd"))
			client, script := newTestClient(t, conn)

			_, err := client.List(context.Background(), &application.ApplicationQuery{})

			assert.Error(t, err)
			assert.Equal(t, code, status.Code(err))
			assert.Equal(t, 1, script.count())
			assert.Equal(t, 0, conn.closer.count())
		})
	}
}

// A cancelled request is the caller going away, not the connection dying —
// shutdown shouldn't spawn connections.
func TestArgoCDClient_DoesNotReconnectForACancelledCaller(t *testing.T) {
	client, script := newTestClient(t, okConn(status.Error(codes.Canceled, "context canceled")))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.List(ctx, &application.ApplicationQuery{})

	assert.Error(t, err)
	assert.Equal(t, 1, script.count())
}

// ArgoCDWrapper fans every request out over pond pools, so one dead connection
// surfaces as a burst of simultaneous failures. They should cost one dial
// between them, not one each.
func TestArgoCDClient_ConcurrentFailuresShareOneReconnect(t *testing.T) {
	dead, alive := okConn(unavailable()), okConn(nil)
	client, script := newTestClient(t, dead, alive)

	var wg sync.WaitGroup
	errs := make([]error, 12)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = client.List(context.Background(), &application.ApplicationQuery{})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "caller %d", i)
	}
	assert.Equal(t, 2, script.count(), "concurrent failures should collapse into one redial")
	assert.Equal(t, 1, dead.closer.count(), "the dead connection should be closed exactly once")
}

// Generation collapsing only holds when the replacement dial succeeds. A failed
// one leaves no current connection, so the callers behind it each dial rather
// than being handed a shared replacement — and, more importantly, rather than
// being locked out of recovery by a redial that happened to fail.
func TestArgoCDClient_FailedRedialDoesNotCollapseLaterAttempts(t *testing.T) {
	client, script := newTestClient(t,
		okConn(unavailable()),
		failedDial(fmt.Errorf("argocd is unreachable")),
		okConn(nil),
	)

	stale, err := client.connection()
	assert.NoError(t, err)

	_, err = client.reconnect(stale)
	assert.Error(t, err)
	assert.Equal(t, 2, script.count())

	// A second caller still holding the stale connection dials for itself,
	// because the failed redial left nothing current to hand it.
	fresh, err := client.reconnect(stale)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), fresh.generation)
	assert.Equal(t, 3, script.count())

	// ...and once one succeeds, collapsing applies again.
	same, err := client.reconnect(stale)
	assert.NoError(t, err)
	assert.Equal(t, fresh, same)
	assert.Equal(t, 3, script.count())
}

func TestArgoCDClient_SurfacesBothErrorsWhenRedialingFails(t *testing.T) {
	lost := unavailable()
	dialErr := fmt.Errorf("argocd is unreachable")
	client, script := newTestClient(t, okConn(lost), failedDial(dialErr), okConn(nil))

	_, err := client.List(context.Background(), &application.ApplicationQuery{})

	assert.ErrorIs(t, err, lost, "the caller should still learn what actually failed")
	assert.ErrorIs(t, err, dialErr, "...and that recovery was attempted and failed")
	assert.Equal(t, 2, script.count())

	// No rate limit on redialing (deferred — see the ADR), so the next request
	// tries again immediately rather than being locked out.
	got, err := client.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, 3, script.count())
}

// A client whose first dial failed is still usable: nothing about a failed
// connection at startup should permanently disable an instance.
func TestArgoCDClient_ConnectsLazilyAfterAFailedFirstDial(t *testing.T) {
	client, script := newTestClient(t, failedDial(fmt.Errorf("argocd is down")), okConn(nil))

	_, err := client.List(context.Background(), &application.ApplicationQuery{})
	assert.Error(t, err)

	got, err := client.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, 2, script.count())
}

func TestArgoCDClient_Close(t *testing.T) {
	conn := okConn(nil)
	client, script := newTestClient(t, conn)

	_, err := client.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err)

	assert.NoError(t, client.Close())
	assert.Equal(t, 1, conn.closer.count(), "Close should release the connection")

	assert.NoError(t, client.Close(), "Close should be safe to call twice")
	assert.Equal(t, 1, conn.closer.count(), "...without closing twice")

	_, err = client.List(context.Background(), &application.ApplicationQuery{})
	assert.ErrorIs(t, err, ErrClientClosed, "a call after Close should fail, not silently reconnect")
	assert.Equal(t, 1, script.count(), "...and certainly not dial a connection nobody will close")
}

// Reconnects have to be countable: the metric is what tells us whether
// redialing ever needs rate-limiting.
func TestArgoCDClient_CountsDials(t *testing.T) {
	client, _ := newTestClient(t, okConn(unavailable()), okConn(nil))

	_, err := client.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err)

	assert.Equal(t, float64(1), testutil.ToFloat64(clientDialsTotal.WithLabelValues(t.Name(), dialReasonInitial, dialResultSuccess)))
	assert.Equal(t, float64(1), testutil.ToFloat64(clientDialsTotal.WithLabelValues(t.Name(), dialReasonReconnect, dialResultSuccess)))
	assert.Equal(t, float64(2), testutil.ToFloat64(clientConnectionGeneration.WithLabelValues(t.Name())))
}

// A reverse proxy in front of ArgoCD (an ingress, a load balancer) can cut a
// gRPC-Web response off partway through. argo-cd's proxy reports that as
// codes.Unknown, which its own retry interceptor ignores, so these reached
// users as a 500 on an otherwise healthy connection. See
// docs/adrs/0028-retry-transient-argocd-transport-failures.md.
func transientErrors() []error {
	return []error{
		status.Error(codes.Unknown, "unexpected EOF"),
		status.Error(codes.Unknown, `Post "http://argocd.example/application.ApplicationService/List": EOF`),
		status.Error(codes.Unknown, `Post "http://argocd.example/application.ApplicationService/List": read tcp 10.0.0.1:5000->10.0.0.2:80: read: connection reset by peer`),
	}
}

func TestArgoCDClient_RetriesTransientTransportFailures(t *testing.T) {
	calls := map[string]func(*ArgoCDClient) (any, error){
		"List": func(c *ArgoCDClient) (any, error) {
			return c.List(context.Background(), &application.ApplicationQuery{})
		},
		"Get": func(c *ArgoCDClient) (any, error) {
			return c.Get(context.Background(), &application.ApplicationQuery{})
		},
		"GetApplicationManifests": func(c *ArgoCDClient) (any, error) {
			return c.GetApplicationManifests(context.Background(), &application.ApplicationManifestQuery{})
		},
	}

	for _, transient := range transientErrors() {
		for name, call := range calls {
			t.Run(fmt.Sprintf("%s/%s", name, status.Convert(transient).Message()), func(t *testing.T) {
				conn := okConn(transient, nil)
				client, script := newTestClient(t, conn)

				got, err := call(client)

				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, 2, conn.apps.calls, "should have retried once")
				assert.Equal(t, 1, script.count(), "the local connection is fine, so nothing should be redialed")
				assert.Equal(t, 0, conn.closer.count())
			})
		}
	}
}

func TestArgoCDClient_GivesUpOnTransientFailuresAfterABoundedNumberOfRetries(t *testing.T) {
	lastErr := status.Error(codes.Unknown, "unexpected EOF")
	conn := okConn(lastErr)
	client, _ := newTestClient(t, conn)

	_, err := client.List(context.Background(), &application.ApplicationQuery{})

	assert.ErrorIs(t, err, lastErr)
	assert.Equal(t, 1+maxTransientRetries, conn.apps.calls)
}

// A caller that has gone away shouldn't be kept waiting for a backoff, or cost
// ArgoCD another request nobody will read.
func TestArgoCDClient_StopsRetryingWhenTheCallerGoesAway(t *testing.T) {
	conn := okConn(status.Error(codes.Unknown, "unexpected EOF"))
	client, _ := newTestClient(t, conn)
	client.backoff = func(int) time.Duration { return time.Hour }

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)

	start := time.Now()
	_, err := client.List(ctx, &application.ApplicationQuery{})

	assert.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second, "should return as soon as the caller cancels")
	assert.Equal(t, 1, conn.apps.calls)
}

// The two recoveries are independent: a truncated response followed by a dead
// connection should still end in an answer.
func TestArgoCDClient_TransientRetryAndReconnectCompose(t *testing.T) {
	dead, alive := okConn(status.Error(codes.Unknown, "unexpected EOF"), unavailable()), okConn(nil)
	client, script := newTestClient(t, dead, alive)

	got, err := client.List(context.Background(), &application.ApplicationQuery{})

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, 2, script.count())
}

func TestArgoCDClient_CountsTransientRetries(t *testing.T) {
	client, _ := newTestClient(t, okConn(status.Error(codes.Unknown, "unexpected EOF"), nil))

	_, err := client.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err)

	assert.Equal(t, float64(1), testutil.ToFloat64(clientRetriesTotal.WithLabelValues(t.Name(), "List", retryReasonTransient)))
}
