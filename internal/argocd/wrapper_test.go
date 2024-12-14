package argocd

// import (
// 	"testing"

// 	"github.com/joho/godotenv"
// 	"github.com/stretchr/testify/assert"
// )

// func TestArgoCDWrapper(t *testing.T) {
// 	t.Run("pool", func(t *testing.T) {
// 		err := godotenv.Load("../../.env")
// 		if err != nil {
// 			t.Fatal(err)
// 		}

// 		client, err := New(&ArgoCDWrapperOptions{
// 			Address:         "https://localhost:808",
// 			Insecure:        true,
// 			AuthTokenEnvVar: "ARGOCD_TOKEN",
// 		})
// 		assert.Nil(t, err)

// 		labels := make(map[string]string)
// 		labels["foo"] = "bar"
// 		labels["apple"] = "banana"

// 		results := client.ListApplicationsByLabels(labels)
// 		assert.Len(t, results, 2)
// 	})
// }
