// Author :		Eric<eehsiao@gmail.com>
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/eehsiao/go-crawler/jobctrl"
	"github.com/gosuri/uiprogress"
	_ "github.com/mattn/go-sqlite3"
)

const (
	maxJobs   = 10
	maxTryCnt = 2
)

var (
	db  *sql.DB
	f   *os.File
	url = "https://law.moj.gov.tw/Law/"

	j      = jobctrl.NewJobCtrl(maxJobs)
	dbLock sync.RWMutex
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
	defer func() {
		f.Close()
		defer db.Close()
	}()

	start := time.Now()
	if nodes, err := retriveCatalogs(url + "LawSearchLaw.aspx"); err == nil {
		uiprogress.Start()
		var (
			wg      sync.WaitGroup
			catalog string
		)
		bar := uiprogress.AddBar(len(nodes)).AppendCompleted().PrependElapsed()
		bar.Width = 50
		bar.PrependFunc(func(b *uiprogress.Bar) string {
			return fmt.Sprintf("%s (%d/%d)", catalog, b.Current(), len(nodes))
		})
		for _, n := range nodes {
			for !j.IncJob() {
				time.Sleep(time.Duration(100) * time.Millisecond)
			}
			wg.Add(1)
			go storeList(n, &wg)
			bar.Incr()
		}
		wg.Wait()
		uiprogress.Stop()
	} else {
		log.Printf("retriveCatalogs error %s\n", err)
	}

	elapsed := time.Since(start)
	log.Printf("took %s\n", elapsed)
}

func retriveCatalogs(u string) (n []*cdp.Node, err error) {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for tryCnt := 0; tryCnt < maxTryCnt; tryCnt++ {
		if err = chromedp.Run(ctx,
			chromedp.Navigate(u),
			chromedp.WaitVisible(`#plLeftCount`),
			chromedp.Nodes(`li > span > a`, &n, chromedp.ByQueryAll),
		); err == nil {
			tryCnt = maxTryCnt
		}
	}

	return
}

func storeList(n *cdp.Node, wg *sync.WaitGroup) {
	defer wg.Done()
	if lists, catalog, err := retriveLists(url + n.AttributeValue("href")); err == nil && len(lists) > 0 {

		catalog = strings.Trim(strings.Trim(catalog, "\n"), " ")

		for _, l := range lists {
			pCode := strings.Split(l.AttributeValue("href"), "=")
			title := l.AttributeValue("title")
			if len(pCode) > 0 {
				dbLock.Lock()
				// fmt.Printf("[%s] : %s : %s\n", catalog, title, pCode[1])
				sql := "INSERT OR REPLACE INTO law_lists(catalog, pcode, name) VALUES (?, ?, ?)"
				if _, err = db.Exec(sql, catalog, pCode[1], title); err != nil {
					log.Fatalf("db.Exec %s error %s\n", sql, err)
				}
				dbLock.Unlock()
			}
		}
	} else {
		log.Printf("[%s]retriveLists error %s\n", n.Dump("", "", false), err)
	}

	j.DecJob()

	return
}

func retriveLists(u string) (n []*cdp.Node, c string, err error) {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for tryCnt := 0; tryCnt < maxTryCnt; tryCnt++ {
		if err = chromedp.Run(ctx,
			chromedp.Navigate(u),
			chromedp.WaitVisible(`tbody`),
			chromedp.Text(`div[class="law-result"] > h3`, &c, chromedp.NodeVisible, chromedp.ByQueryAll),
			chromedp.Nodes(`#hlkLawName`, &n, chromedp.ByQueryAll),
		); err == nil {
			tryCnt = maxTryCnt
		}
	}
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
