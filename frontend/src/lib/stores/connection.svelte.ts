// Whether this page still talks to the codec run that served it. Each
// run has a new token, so after a restart every request is refused until
// the page reloads; the shell then shows a reload banner.

class Connection {
  restarted = $state(false)
}

export const connection = new Connection()
