package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/antonholmquist/jason"
	"github.com/apex/gateway/v2"
)

type CalendarItem struct {
	Title        map[string]string `json:"title"`
	DisplayValue map[string]string `json:"displayValue"`
	URL          string            `json:"url"`
	Ref          string            `json:"ref"`
	Category     string            `json:"category"`
}

type CalendarResponse struct {
	CalendarItems []CalendarItem `json:"calendar_items"`
}

var (
	port = flag.Int("port", -1, "specify a port")
)

func main() {
	flag.Parse()

	http.HandleFunc("/api/daf", daf)
	listener := gateway.ListenAndServe
	portStr := "n/a"

	if *port != -1 {
		portStr = fmt.Sprintf(":%d", port)
		listener = http.ListenAndServe
		http.Handle("/", http.FileServer(http.Dir("./public")))
	}

	log.Fatal(listener(portStr, nil))
}

func daf(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/html")
	w.Write([]byte(getDaf()))
}

func getDaf() string {
	url := "https://www.sefaria.org/api/calendars?diaspora=1&custom=ashkenazi"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	var responseData CalendarResponse
	if err := json.Unmarshal(body, &responseData); err != nil {
		panic(err)
	}

	var mishna, rashi, tosafot string

	for _, item := range responseData.CalendarItems {
		if item.Title["en"] != "Daf Yomi" {
			continue
		}
		mishna = getText(item.Ref)
		rashi = getCommentary(item.Ref, "Rashi")
		tosafot = getCommentary(item.Ref, "Tosafot")
	}
	return fmt.Sprintf(`
    <article class="daf">
    <div class="mishna">
      <p dir="rtl">
        %s
      </p>
    </div>
    <aside>
      <div class="rashi">
        <p dir="rtl">
          %s
        </p>
      </div>
      <div>
        <p dir="rtl">
          %s
        </p>
      </div>
    </aside>
  </article>
`, mishna, rashi, tosafot)
}

func getCommentary(ref, commentator string) string {
	newRef := fmt.Sprintf("%s on %s", commentator, ref)
	url := fmt.Sprintf("https://www.sefaria.org/api/v3/texts/%s?return_format=strip_only_footnotes", newRef)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic("failed to create request")
	}
	req.Header.Add("accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic("request failed")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic("failed to read response body")
	}

	responseData, err := jason.NewObjectFromBytes(body)
	if err != nil {
		panic(err)
	}

	versions, err := responseData.GetObjectArray("versions")
	if err != nil {
		panic(err)
	}

	ret := ""
	for _, version := range versions {
		texts, err := version.GetValueArray("text")
		if err != nil {
			panic(err)
		}
		for _, arr := range texts {
			t1, err := arr.Array()
			if err != nil {
				panic(err)
			}
			for _, t := range t1 {
				t2, err := t.Array()
				if err != nil {
					panic(err)
				}
				for _, text := range t2 {
					str, err := text.String()
					if err != nil {
						ret += "\n<br>"
					}
					ret += fmt.Sprintf("%s<br>", str)
				}
			}
		}
	}

	return ret
}

func getText(ref string) string {
	url := fmt.Sprintf("https://www.sefaria.org/api/v3/texts/%s?return_format=strip_only_footnotes", ref)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic("failed to create request")
	}
	req.Header.Add("accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic("request failed")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic("failed to read response body")
	}

	responseData, err := jason.NewObjectFromBytes(body)
	if err != nil {
		panic(err)
	}

	versions, err := responseData.GetObjectArray("versions")
	if err != nil {
		panic(err)
	}

	ret := ""
	for _, version := range versions {
		texts, err := version.GetValueArray("text")
		if err != nil {
			panic(err)
		}
		for _, text := range goToLowestLevel(texts) {
			str, err := text.String()
			if err != nil {
				panic(err)
			}
			ret += fmt.Sprintf("%s<br>", str)
		}
	}

	return ret
}

func goToLowestLevel(arrs []*jason.Value) []*jason.Value {
	for _, text := range arrs {
		arr, err := text.Array()
		if err != nil {
			return arrs
		}
		return goToLowestLevel(arr)
	}

	return nil
}
