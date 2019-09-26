// Author :		Eric<eehsiao@gmail.com>
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	_ "github.com/mattn/go-sqlite3"
)

const (
	maxJob = 10
)

var (
	db           *sql.DB
	err          error
	url          = "https://law.moj.gov.tw/Law/"
	nodes, lists []*cdp.Node

	jobLock sync.RWMutex
	jobCnt  int
)

func incJob() int {
	jobLock.Lock()
	defer jobLock.Unlock()

	jobCnt++

	return jobCnt
}

func decJob() int {
	jobLock.Lock()
	defer jobLock.Unlock()

	jobCnt--

	return jobCnt
}

func getJobCount() int {
	jobLock.RLock()
	defer jobLock.RUnlock()

	return jobCnt
}

func init() {
	if db, err = openSqlite(20, 2); err != nil {
		panic(err.Error())
	}

	if err = initTable(db); err != nil {
		panic(err.Error())
	}
}

func main() {
	defer db.Close()

	start := time.Now()
	r := new(big.Int)
	fmt.Println(r.Binomial(1000, 10))

	nodes, err = retriveCatalogs(url + "LawSearchLaw.aspx")

	for _, n := range nodes {
		for getJobCount() >= maxJob {
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
		incJob()
		go storeList(n)

	}

	time.Sleep(time.Duration(10) * time.Millisecond)

	for getJobCount() > 0 {
		time.Sleep(time.Duration(10) * time.Millisecond)
	}

	elapsed := time.Since(start)
	log.Printf("took %s\n", elapsed)
}

func storeList(n *cdp.Node) {

	var catalog string

	lists, catalog, err = retriveLists(url + n.AttributeValue("href"))
	catalog = strings.Trim(strings.Trim(catalog, "\n"), " ")
	fmt.Printf("%s : [%s]\n", n.AttributeValue("href"), catalog)

	for _, l := range lists {
		pCode := strings.Split(l.AttributeValue("href"), "=")
		title := l.AttributeValue("title")
		if len(pCode) > 0 {
			// fmt.Printf("[%s] : %s : %s\n", catalog, title, pCode[1])
			sql := "INSERT OR REPLACE INTO lawl_list(catalog, pcode, name) VALUES ('" + catalog + "', '" + pCode[1] + "', '" + title + "')"
			_, err = db.Exec(sql)
		}
	}

	decJob()
}

func retriveCatalogs(u string) (n []*cdp.Node, err error) {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = chromedp.Run(ctx,
		chromedp.Navigate(u),
		chromedp.WaitVisible(`#plLeftCount`),
		chromedp.Nodes(`li > span > a`, &n, chromedp.ByQueryAll),
	)

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

	err = chromedp.Run(ctx,
		chromedp.Navigate(u),
		chromedp.WaitVisible(`tbody`),
		chromedp.Text(`div[class="law-result"] > h3`, &c, chromedp.NodeVisible, chromedp.ByQueryAll),
		chromedp.Nodes(`#hlkLawName`, &n, chromedp.ByQueryAll),
	)

	return
}

func openSqlite(max, min int) (db *sql.DB, err error) {
	db, err = sql.Open("sqlite3", "./lawlists.sqlite")
	if err != nil {
		return
	}

	db.SetMaxOpenConns(max)
	db.SetMaxIdleConns(min)

	return
}

func initTable(db *sql.DB) (err error) {
	sql := `
	CREATE TABLE IF NOT EXISTS lawl_list (
		id integer PRIMARY KEY AUTOINCREMENT NOT NULL,
		catalog varchar(255) NOT NULL,
		pcode char(8),
		name varchar(255) NOT NULL
	);
	`
	_, err = db.Exec(sql)

	return
}
