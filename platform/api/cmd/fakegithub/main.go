// Command fakegithub serves the GitHub OAuth test double for the Compose e2e check.
// Test-only: it is built into a separate image stage and never deployed.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/fakegithub"
)

func main() {
	id, _ := strconv.ParseInt(os.Getenv("FAKE_USER_ID"), 10, 64)
	srv := fakegithub.New(os.Getenv("GITHUB_CLIENT_ID"), os.Getenv("GITHUB_CLIENT_SECRET"),
		fakegithub.User{ID: id, Login: os.Getenv("FAKE_USER_LOGIN")})
	log.Fatal(http.ListenAndServe(":9999", srv.Handler()))
}
