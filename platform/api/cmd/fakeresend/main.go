// Command fakeresend serves the Resend API test double for the Compose e2e check.
// Test-only: it is built into a separate image stage and never deployed.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/fakeresend"
)

func main() {
	log.Fatal(http.ListenAndServe(":9998", fakeresend.New(os.Getenv("FAKE_RESEND_API_KEY")).Handler()))
}
