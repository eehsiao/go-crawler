// Author :		Eric<eehsiao@gmail.com>
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/eehsiao/go-crawler/jobctrl"
	"github.com/gocolly/colly"
	"github.com/gosuri/uiprogress"
	_ "github.com/mattn/go-sqlite3"
)

const (
	maxJobs = 10
)

var (
	db  *sql.DB
	f   *os.File
	url = "https://law.moj.gov.tw/Law/"

	j = jobctrl.NewJobCtrl(maxJobs)
)

func init() {
	var err error
	if f, err = os.OpenFile("error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
		panic(err.Error())
	}
	log.SetOutput(f)

	if db, err = openSqlite(20, 2); err != nil {
		panic(err.Error())
	}

	if err = initTable(db); err != nil {
		panic(err.Error())
	}
}

func main() {
	type item struct {
		title string
		link  string
	}

	var (
		catalogs []item
		catalog  string
	)
	start := time.Now()
	l := colly.NewCollector(
		// colly.Debugger(&debug.LogDebugger{}),
		colly.Async(true),
	)
	l.Limit(&colly.LimitRule{
		Parallelism: maxJobs,
		Delay:       150 * time.Microsecond,
	})
	c := l.Clone()

	// retriveCatalogs
	c.OnHTML(`li > span > a[href]`, func(e *colly.HTMLElement) {
		catalogs = append(catalogs, item{title: e.Text, link: e.Attr("href")})

	})
	c.Visit(url + "LawSearchLaw.aspx")
	c.Wait()

	uiprogress.Start()

	defer func() {
		uiprogress.Stop()
		f.Close()
		defer db.Close()
	}()

	bar := uiprogress.AddBar(len(catalogs)).AppendCompleted().PrependElapsed()
	bar.Width = 50
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return fmt.Sprintf("(%d/%d)", b.Current(), len(catalogs))
	})

	//retriveLists
	l.OnHTML("body", func(e *colly.HTMLElement) {
		if em := e.DOM.Find(`div[class="law-result"] > h3`).Text(); em != "" {
			catalog = strings.Trim(strings.Trim(em, "\n"), " ")
			e.ForEach("#hlkLawName", func(_ int, el *colly.HTMLElement) {
				pCode := strings.Split(el.Attr("href"), "=")
				if len(pCode) > 0 {
					// fmt.Printf("%s : %s : %s : %s\n", catalog, el.Text, el.Attr("href"), pCode[1])
					sql := "INSERT OR REPLACE INTO law_lists(catalog, pcode, name) VALUES ('" + catalog + "', '" + pCode[1] + "', '" + el.Text + "')"
					if _, err := db.Exec(sql); err != nil {
						log.Fatalf("db.Exec %s error %s\n", sql, err)
					}
				}
			})
		}

	})

	l.OnScraped(func(r *colly.Response) {
		bar.Incr()
		j.DecJob()
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Fatalf("error:", err)
	})

	for _, v := range catalogs {
		for !j.IncJob() {
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
		l.Visit(url + v.link)
	}
	l.Wait()
	elapsed := time.Since(start)
	fmt.Printf("took %s\n", elapsed)
}

func openSqlite(max, min int) (db *sql.DB, err error) {
	db, err = sql.Open("sqlite3", "./roc_laws.sqlite")
	if err != nil {
		return
	}

	db.SetMaxOpenConns(max)
	db.SetMaxIdleConns(min)

	return
}

func initTable(db *sql.DB) (err error) {
	sql := `
	CREATE TABLE IF NOT EXISTS law_lists (
		id integer PRIMARY KEY AUTOINCREMENT NOT NULL,
		catalog varchar(255) NOT NULL,
		pcode char(8),
		name varchar(255) NOT NULL
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx01_law_lists_pcode ON law_lists (pcode);
	`
	_, err = db.Exec(sql)

	return
}
