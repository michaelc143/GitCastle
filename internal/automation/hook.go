package automation

import (
	"fmt"
	"os/exec"
	"strings"
)

// PushHook is installed as a post-receive hook in every new bare repository.
// It forwards branch updates to GitCastle's HTTP API, which fans events out
// to webhooks and CI.
const HookScript = `#!/bin/sh
# Installed by GitCastle. Forwards push notifications to the server API.
while read oldrev newrev refname; do
  branch="${refname#refs/heads/}"
  [ "$branch" = "$refname" ] && continue # not a branch (tag etc.)
  curl -s -o /dev/null -m 5 \
    -X POST "$GITCASTLE_URL/api/v1/internal/notify-push" \
    -H "Content-Type: application/json" \
    -H "X-GitCastle-Internal-Token: $GITCASTLE_TOKEN" \
    -d "{\"owner\":\"$GITCASTLE_OWNER\",\"name\":\"$GITCASTLE_NAME\",\"branch\":\"$branch\",\"old_hash\":\"$oldrev\",\"new_hash\":\"$newrev\"}" || true
done
`

// HookEnv describes per-repository values baked into the hook file.
type HookEnv struct {
	ServerURL string
	Token     string
	Owner     string
	Name      string
}

// InstallPostReceiveHook writes the hook into a bare repository.
func InstallPostReceiveHook(barePath string, env HookEnv) error {
	script := fmt.Sprintf("GITCASTLE_URL=%q\nGITCASTLE_TOKEN=%q\nGITCASTLE_OWNER=%q\nGITCASTLE_NAME=%q\n%s",
		env.ServerURL, env.Token, env.Owner, env.Name, HookScript)
	return exec.Command("/bin/sh", "-c",
		fmt.Sprintf("printf '%%s\\n' %s > %s && chmod +x %s",
			quoteShell(script), quoteShell(barePath+"/hooks/post-receive"), quoteShell(barePath+"/hooks/post-receive")),
	).Run()
}

func quoteShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
