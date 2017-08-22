package main

import (
	"fmt"
	"net/http"
	"html/template"
	_ "github.com/mattn/go-sqlite3"
	"database/sql"
)

type page struct {
	Name string
	DBstatus bool
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

	fmt.Println(http.ListenAndServe(":8080", nil))

}
