package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/antonholmquist/jason"
	"github.com/carlmjohnson/gateway"
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
	portStr := ":3000"

	if *port != -1 {
		portStr = fmt.Sprintf(":%d", port)
		listener = http.ListenAndServe
		http.Handle("/", http.FileServer(http.Dir("./public")))
	}

	log.Fatal(listener(portStr, nil))
}

func daf(w http.ResponseWriter, r *http.Request) {
	ret, err := getDaf()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("content-type", "text/html")
	w.Write([]byte(ret))
}

func getDaf() (string, error) {
	url := "https://www.sefaria.org/api/calendars?diaspora=1&custom=ashkenazi"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var responseData CalendarResponse
	if err := json.Unmarshal(body, &responseData); err != nil {
		return "", err
	}

	var mishna, rashi, tosafot string

	for _, item := range responseData.CalendarItems {
		if item.Title["en"] != "Daf Yomi" {
			continue
		}
		mishna, err = getText(item.Ref)
		if err != nil {
			return "", err
		}
		rashi, err = getCommentary(item.Ref, "Rashi")
		if err != nil {
			return "", err
		}
		tosafot, err = getCommentary(item.Ref, "Tosafot")
		if err != nil {
			return "", err
		}
	}
	script := fmt.Sprintf(`<script>
    const {top: dafTop} = document.querySelector(".daf").getBoundingClientRect();
    const aside = document.querySelector("aside");
    const mishnaRect = document.querySelector(".mishna").getBoundingClientRect();
    const rashi = aside.querySelector(".rashi");

    const tosafotFloater = document.createElement("div");
    tosafotFloater.style = %s
float: right;
width: calc((${mishnaRect.width}px - var(--column-gap))/2);
height: ${mishnaRect.height + mishnaRect.y - dafTop}px;
shape-outside: inset(${mishnaRect.y - dafTop}px 0 0 0);
margin-left: var(--column-gap);\%s;

    rashi.after(tosafotFloater);

    const rashiFloater = document.createElement("div");
    rashiFloater.style = %s
float: left;
width: calc((${mishnaRect.width}px - var(--column-gap))/2);
height: ${mishnaRect.height + mishnaRect.y - dafTop}px;
shape-outside: inset(${mishnaRect.y - dafTop}px 0 0 0);
margin-right: var(--column-gap);%s;

    rashi.prepend(rashiFloater);
  </script>
`, "`", "`", "`", "`")
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
  %s
`, mishna, rashi, tosafot, script), nil
}

func getCommentary(ref, commentator string) (string, error) {
	newRef := fmt.Sprintf("%s on %s", commentator, ref)
	url := fmt.Sprintf("https://www.sefaria.org/api/v3/texts/%s?return_format=strip_only_footnotes", newRef)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	responseData, err := jason.NewObjectFromBytes(body)
	if err != nil {
		return "", err
	}

	versions, err := responseData.GetObjectArray("versions")
	if err != nil {
		return "", err
	}

	ret := ""
	for _, version := range versions {
		texts, err := version.GetValueArray("text")
		if err != nil {
			return "", err
		}
		for _, arr := range texts {
			t1, err := arr.Array()
			if err != nil {
				return "", err
			}
			for _, t := range t1 {
				t2, err := t.Array()
				if err != nil {
					return "", err
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

	return ret, nil
}

func getText(ref string) (string, error) {
	url := fmt.Sprintf("https://www.sefaria.org/api/v3/texts/%s?return_format=strip_only_footnotes", ref)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	responseData, err := jason.NewObjectFromBytes(body)
	if err != nil {
		return "", err
	}

	versions, err := responseData.GetObjectArray("versions")
	if err != nil {
		return "", err
	}

	ret := ""
	for _, version := range versions {
		texts, err := version.GetValueArray("text")
		if err != nil {
			return "", err
		}
		for _, text := range goToLowestLevel(texts) {
			str, err := text.String()
			if err != nil {
				return "", err
			}
			ret += fmt.Sprintf("%s<br>", str)
		}
	}

	return ret, nil
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
