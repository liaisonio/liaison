package web

import (
	_ "embed"
	"os"
	"strings"
)

//go:embed shell-integration/bash.sh
var webSSHBashIntegration string

type webSSHShellStarter interface {
	Shell() error
	Start(string) error
}

func startWebSSHShell(session webSSHShellStarter) error {
	if !strings.EqualFold(os.Getenv("LIAISON_WEBSSH_SHELL_INTEGRATION"), "true") {
		return session.Shell()
	}
	return session.Start(webSSHShellBootstrap())
}

func shellLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// Only embedded, trusted script content enters this command. No browser input or
// credential is interpolated. Run the bootstrap under sh even for non-POSIX users.
func webSSHShellBootstrap() string {
	bootstrap := `case "${SHELL##*/}" in
bash)
  if "$SHELL" -c '(( BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 4) ))'; then
    umask 077
    liaison_rc=$(mktemp "${TMPDIR:-/tmp}/liaison-bash.XXXXXXXX") || exec "$SHELL" -l
    if printf '%s\n' ` + shellLiteral(webSSHBashIntegration) + ` > "$liaison_rc"; then
      "$SHELL" --rcfile "$liaison_rc" -i
      liaison_status=$?
      rm -f -- "$liaison_rc"
      exit "$liaison_status"
    fi
    rm -f -- "$liaison_rc"
  fi
  ;;
esac
exec "${SHELL:-/bin/sh}" -l`
	return "exec /bin/sh -c " + shellLiteral(bootstrap)
}
