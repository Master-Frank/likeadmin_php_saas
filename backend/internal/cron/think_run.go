package cron

import "strings"

// RunListen holds php-think-run flags. Host/port default to ThinkPHP's
// 0.0.0.0:8000; root is the public directory when given.
type RunListen struct {
	Host string
	Port string
	Root string
}

func ParseRunArgs(args []string) RunListen {
	out := RunListen{Host: "0.0.0.0", Port: "8000"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		take := func() string {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case a == "--host" || a == "-H":
			if v := take(); v != "" {
				out.Host = v
			}
		case strings.HasPrefix(a, "--host="):
			out.Host = strings.TrimPrefix(a, "--host=")
		case strings.HasPrefix(a, "-H="):
			out.Host = strings.TrimPrefix(a, "-H=")
		case a == "--port" || a == "-p":
			if v := take(); v != "" {
				out.Port = v
			}
		case strings.HasPrefix(a, "--port="):
			out.Port = strings.TrimPrefix(a, "--port=")
		case strings.HasPrefix(a, "-p="):
			out.Port = strings.TrimPrefix(a, "-p=")
		case a == "--root" || a == "-r":
			if v := take(); v != "" {
				out.Root = v
			}
		case strings.HasPrefix(a, "--root="):
			out.Root = strings.TrimPrefix(a, "--root=")
		case strings.HasPrefix(a, "-r="):
			out.Root = strings.TrimPrefix(a, "-r=")
		}
	}
	if out.Host == "" {
		out.Host = "0.0.0.0"
	}
	if out.Port == "" {
		out.Port = "8000"
	}
	return out
}

func (r RunListen) Addr() string {
	return r.Host + ":" + r.Port
}
