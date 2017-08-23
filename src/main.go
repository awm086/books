package main

import (
	"fmt"
	"net/http"
	"html/template"
	_ "github.com/mattn/go-sqlite3"
	"database/sql"
	"encoding/json"
	"net/url"
	"io/ioutil"
	"encoding/xml"
)

type page struct {
	Name string
	DBstatus bool
}

type searchResult struct {
	Title string  `xml:"title,attr"`
	Author string `xml:"author,attr"`
	Year string `xml:"hyr,attr"`
	ID string   `xml:"owi,attr"`
}

type classifySearchResp struct {
	Results []searchResult `xml:"works>work"`
}

func main() {
	template := template.Must(template.ParseFiles("templates/index.html"))
	p := new(page);
	p.Name = "GOPHER";

	// db
	db,_ := sql.Open("sqlite3", "goweb.dev")
	p.DBstatus = db.Ping() == nil

	fmt.Println("hello world")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name");
		if  name != "" {
			p.Name = name
		}

		err := template.ExecuteTemplate(w, "index.html", p)
		if (err != nil) {
			http.Error(w, "something went wrong", http.StatusInternalServerError)
		}
	})


	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var results []searchResult
		var err error

		if results, err = search(r.FormValue("search")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(results); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	fmt.Println(http.ListenAndServe(":8080", nil))


}


func search (query string) ([]searchResult, error) {
	var resp *http.Response
	var err error

	if resp, err = http.Get("http://classify.oclc.org/classify2/Classify?&summary=true&title=" + url.QueryEscape(query)); err != nil {
		return []searchResult{}, err
	}

	var body []byte
	defer resp.Body.Close()
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return []searchResult{}, err
	}

	var c classifySearchResp
	err = xml.Unmarshal(body, &c)

	return c.Results, err

}