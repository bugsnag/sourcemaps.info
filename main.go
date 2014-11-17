package main

import (
	"io"
	"log"
	"net"
	"net/http"

	"github.com/bugsnag/bugsnag-go"
)

func main() {

	bugsnag.Configure(bugsnag.Configuration{
		APIKey: "953611040d6cfb7f10a9aec0ae81b845",
	})

	http.Handle("/bower_components/", http.StripPrefix("/bower_components/", http.FileServer(http.Dir("bower_components"))))

	http.HandleFunc("/get/", proxy)

	http.Handle("/", http.FileServer(http.Dir("public")))

	log.Println("Listening on :4000")

	http.ListenAndServe(":4000", bugsnag.Handler(nil))
}

func proxy(w http.ResponseWriter, r *http.Request) {

	req, err := http.NewRequest("GET", r.URL.Query().Get("url"), nil)
	if err != nil {
		log.Fatalln(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	// Make it easy for upstreams to filter out traffic from sourcemaps.info
	// We should also deploy this with a static outbound IP.
	req.Header.Set("User-Agent", "proxy.sourcemaps.info/v1.0 (conrad@bugsnag.com)")

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	req.Header.Set("X-Forwarded-For", ip)
	req.Header.Set("X-SourceMapsInfo-User", r.RemoteAddr)

	log.Printf("Fetching %s for %s\n", r.URL.Query().Get("url"), r.RemoteAddr)

	resp, err := (&http.Client{}).Do(req)

	if err != nil {
		w.Header().Set("X-Proxy-Error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
		return
	}

	if resp.Header.Get("X-SourceMap") != "" {
		w.Header().Set("X-SourceMap", resp.Header.Get("X-SourceMap"))
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
