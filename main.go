package main

import (
	"fmt"
	"log"

	"github.com/mmcdole/gofeed"
)

func main() {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL("http://feeds.twit.tv/twit.xml")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(feed.Title)

	firstFive := feed.Items[:min(5, len(feed.Items))]
	for _, item := range firstFive {
		fmt.Println(item.Title, item.Link)
	}
}
