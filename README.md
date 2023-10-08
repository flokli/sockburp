# sockburp

```
Usage: sockburp <listen-address>

Duplicate requests received over unix sockets and send them to two backends

Arguments:
  <listen-address>    The path of the unix domain socket to listen on

Flags:
  -h, --help                            Show context-sensitive help.
      --log-level="info"                The log level to log with
      --first-remote-address=STRING     The first (and authoritative) socket address to forward requests to
      --second-remote-address=STRING    The second socket address responses are compared against
      --pcap-path=STRING                Path to a .pcap file where differing responses (and the original request) are written to.
```