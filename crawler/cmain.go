/*
	GCSE Crawler background program.
*/
package main

import (
	"log"
	"runtime"
	"sync"
	"time"
	
	"github.com/daviddengcn/gcse"
	"github.com/daviddengcn/go-villa"
)

const (
	fnOldDocDB  = "docdb"
	fnDocDB     = "packed-docdb"
	fnCrawlerDB = "crawler"
)

var (
	DocDBPath     villa.Path
	CrawlerDBPath villa.Path
	AppStopTime   time.Time
)

func init() {
	DocDBPath = gcse.DataRoot.Join(fnDocDB)
	CrawlerDBPath = gcse.DataRoot.Join(fnCrawlerDB)
}

func syncDatabases() {
	gcse.DumpMemStats()
	log.Printf("Synchronizing databases to disk...")
	if err := docDB.Sync(); err != nil {
		log.Printf("docDB.Sync failed: %v", err)
	}
	if err := cPackageDB.Sync(); err != nil {
		log.Printf("cPackageDB.Sync failed: %v", err)
	}
	if err := cPersonDB.Sync(); err != nil {
		log.Printf("cPersonDB.Sync failed: %v", err)
	}
	gcse.DumpMemStats()
	runtime.GC()
	gcse.DumpMemStats()
}

func syncLoop(wg *sync.WaitGroup) {
	for AppStopTime.Sub(time.Now()) > gcse.CrawlerSyncGap {
		time.Sleep(gcse.CrawlerSyncGap)
		syncDatabases()
	}
	wg.Done()
}

func dumpingStatusLoop() {
	for time.Now().Before(AppStopTime) {
		gcse.DumpMemStats()
		time.Sleep(10 * time.Minute)
	}
}

func loadDocDB(oldDocDBPath, docDBPath villa.Path) (docDB gcse.PackedDocDB) {
	log.Printf("loadDocDB: old from %v, current from %v", oldDocDBPath, docDBPath)
	oldDocDB := gcse.NewMemDB(oldDocDBPath, gcse.KindDocDB)
	docDB = gcse.PackedDocDB{gcse.NewMemDB(docDBPath, gcse.KindDocDB)}
	all, put := 0, 0
	if err := oldDocDB.Iterate(func(pkg string, data interface{}) error {
		all ++
		var info gcse.DocInfo
		if docDB.Get(pkg, &info) {
			return nil
		}
		
		docDB.Put(pkg, data.(gcse.DocInfo))
		put ++
		return nil
	}); err != nil {
		log.Fatalf("oldDocDB.Iterate failed: %v", err)
	}
	
	log.Printf("All %d entries in old DocDB, %d put!", all, put)
	
	oldDocDB = nil
	runtime.GC()
	
	
	return docDB
}

func main() {
	log.Println("crawler started...")

	AppStopTime = time.Now().Add(10 * time.Minute)

	docDB = loadDocDB(gcse.DataRoot.Join(fnOldDocDB), DocDBPath)

	
	cPackageDB = gcse.NewMemDB(CrawlerDBPath, kindPackage)
	cPersonDB = gcse.NewMemDB(CrawlerDBPath, kindPerson)

	syncDatabases()

	go dumpingStatusLoop()

	var wg sync.WaitGroup
	wg.Add(1)
	go syncLoop(&wg)

	crawlEntriesLoop()

	// dump docDB
	if err := gcse.DBOutSegments.ClearUndones(); err != nil {
		log.Printf("DBOutSegments.ClearUndones failed: %v", err)
	}

	if err := dumpDB(); err != nil {
		log.Printf("dumpDB failed: %v", err)
	}

	wg.Wait()
	log.Println("crawler stopped...")
}
