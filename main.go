package main

import (
	"fmt"
	"github.com/gocolly/colly"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	// 列表采集器
	listCollector := colly.NewCollector(
		// Visit only domains: hackerspaces.org, wiki.hackerspaces.org
		colly.AllowedDomains("www.mzitu.com"),
		//colly.Async(true),
		colly.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.111 Safari/537.36"),
	)

	registerCollector(listCollector)
	listCollector.DisableCookies()

	// 详情采集器
	detailCollector := listCollector.Clone()
	registerCollector(detailCollector)

	// 设置代理
	//if p, err := proxy.RoundRobinProxySwitcher(
	//	//"socks5://127.0.0.1:1337",
	//	//"socks5://127.0.0.1:1338",
	//	"http://119.18.197.98:3128",
	//	"http://113.100.209.195:3128",
	//); err == nil {
	//	detailCollector.SetProxyFunc(p)
	//	listCollector.SetProxyFunc(p)
	//}

	// 限流
	_ = detailCollector.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 4, Delay: 200 * time.Millisecond})
	_ = listCollector.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2, Delay: 500 * time.Millisecond})

	// 抓取列表结果数据钩子函数
	listCollector.OnHTML("#pins li", func(e *colly.HTMLElement) {
		imgDom := e.DOM.Find(".lazy")

		img := imgDom.AttrOr("data-original", "")
		title := imgDom.AttrOr("alt", "")

		linkDom := e.DOM.Find("span a")
		link := linkDom.AttrOr("href", "")

		timeDom := e.DOM.Find(".time")
		timeText := timeDom.Text()

		//// Print link
		fmt.Printf("Link found: %s -> %s -> %s -> %s \n", title, timeText, img, link)
		// Visit link found on page
		// Only those links are visited which are in AllowedDomains
		_ = detailCollector.Visit(e.Request.AbsoluteURL(link))
	})

	// 抓取下一页数据钩子
	listCollector.OnHTML(".next", func(e *colly.HTMLElement) {
		nextUrl := e.Attr("href")
		_ = listCollector.Visit(e.Request.AbsoluteURL(nextUrl))
	})

	// 详情结果数据钩子
	detailCollector.OnHTML(".content", func(e *colly.HTMLElement) {
		categoryDom := e.DOM.Find(".main-meta a")
		category := categoryDom.Text()

		imgDom := e.DOM.Find("img.blur")
		imgUrl := imgDom.AttrOr("src", "")
		title := imgDom.AttrOr("alt", "")

		nextDom := e.DOM.Find(".pagenavi a").Last()

		// 文件下载
		lastIndex := strings.LastIndex(imgUrl, "/")
		length := len(imgUrl)
		fileDownload(category+"/"+title, imgUrl[lastIndex:length], imgUrl)

		// 是否还有下一页
		if strings.Index(nextDom.Text(), "下一页") != -1 {
			nextUrl := nextDom.AttrOr("href", "")
			fmt.Printf("detail: %s -> %s -> %s -> %s\n", category, title, imgUrl, nextUrl)
			_ = detailCollector.Visit(e.Request.AbsoluteURL(nextUrl))
		} else {
			fmt.Printf("detail: %s -> %s\n", category, imgUrl)
		}
	})

	// Start scraping on https://hackerspaces.org
	_ = listCollector.Visit("https://www.mzitu.com/page/3/")

	// 等待爬虫结束
	listCollector.Wait()
	detailCollector.Wait()
}

func registerCollector(collector *colly.Collector) {
	// Before making a request print "Visiting ..."
	collector.OnRequest(func(r *colly.Request) {
		fmt.Printf("Visiting: %s \n", r.URL.String())
	})

	// Set error handler
	collector.OnError(func(r *colly.Response, err error) {
		fmt.Printf("Request URL: %s, failed with response: %v, Error: %v \n", r.Request.URL, r, err)
	})
}

func fileDownload(filepath string, filename string, imgUrl string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("发生异常，地址忽略: %s", imgUrl)
		}
	}()

	// 创建目录
	_ = os.MkdirAll(filepath, os.ModePerm)

	// 读取字节流
	req, err := http.NewRequest("GET", imgUrl, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.111 Safari/537.36")
	req.Header.Set("referer", "https://www.mzitu.com/")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	// 创建一个文件用于保存
	out, err := os.Create(filepath + "/" + filename)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	// 然后将响应流和文件流对接起来
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		panic(err)
	}
}
