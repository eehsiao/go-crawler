// Author :		Eric<eehsiao@gmail.com>
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/eehsiao/go-crawler/jobctrl"
	"github.com/gocolly/colly"
	"github.com/gosuri/uiprogress"
	_ "github.com/mattn/go-sqlite3"
)

const (
	maxJobs = 10
)

type item struct {
	title string
	link  string
}

var (
	db       *sql.DB
	f        *os.File
	url      = "https://law.moj.gov.tw/Law/"
	catalogs []item
	j        = jobctrl.NewJobCtrl(maxJobs)
	wg       sync.WaitGroup
	dbLock   sync.RWMutex
)

func init() {
	var err error
	if f, err = os.OpenFile("error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
		panic(err.Error())
	}
	log.SetOutput(f)

	if db, err = openSqlite(5, 2); err != nil {
		panic(err.Error())
	}

	if err = initTable(db); err != nil {
		panic(err.Error())
	}
}

func main() {
	defer func() {
		f.Close()
		defer db.Close()
	}()

	start := time.Now()
	c := colly.NewCollector(
		// colly.Debugger(&debug.LogDebugger{}),
		colly.Async(true), //non-blocked
	)

	// retriveCatalogs
	c.OnHTML(`li > span > a[href]`, func(e *colly.HTMLElement) {
		catalogs = append(catalogs, item{title: e.Text, link: e.Attr("href")})

	})
	c.Visit(url + "LawSearchLaw.aspx")
	c.Wait()

	uiprogress.Start()
	bar := uiprogress.AddBar(len(catalogs)).AppendCompleted().PrependElapsed()
	bar.Width = 50
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return fmt.Sprintf("(%d/%d)", b.Current(), len(catalogs))
	})

	//retriveLists
	for _, v := range catalogs {
		for !j.IncJob() {
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
		wg.Add(1)
		go storeList(url+v.link, bar, &wg)
	}
	wg.Wait()
	uiprogress.Stop()

	elapsed := time.Since(start)
	fmt.Printf("took %s\n", elapsed)
}

func storeList(u string, bar *uiprogress.Bar, wg *sync.WaitGroup) (err error) {
	defer func() {
		bar.Incr()
		j.DecJob()
		wg.Done()
	}()

	// blocked in goroutine
	l := colly.NewCollector()
	l.OnHTML("body", func(e *colly.HTMLElement) {
		if em := e.DOM.Find(`div[class="law-result"] > h3`).Text(); em != "" {
			catalog := strings.Trim(strings.Trim(em, "\n"), " ")
			e.ForEach("#hlkLawName", func(_ int, el *colly.HTMLElement) {
				pCode := strings.Split(el.Attr("href"), "=")
				if len(pCode) > 0 {
					dbLock.Lock()
					// fmt.Printf("%s : %s : %s : %s\n", catalog, el.Text, el.Attr("href"), pCode[1])
					sql := "INSERT OR REPLACE INTO law_lists(catalog, pcode, name) VALUES (?, ?, ?)"
					if _, err = db.Exec(sql, catalog, pCode[1], el.Text); err != nil {
						log.Fatalf("db.Exec %s error %s\n", sql, err)
					}
					dbLock.Unlock()
				}
			})
		}

	})

	l.OnError(func(_ *colly.Response, e error) {
		log.Fatalf("error:", e)
		err = e
	})

	l.Visit(u)
	return
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
