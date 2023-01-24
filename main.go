package main

import (
	"html/template"
	"net/http"
	"time"

	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type TopPost struct {
	ID          int
	Title       string
	Score       int
	Descendants int
	Time        time.Time
}

type Item struct {
	ID          int
	Title       string
	Text        string
	By          string
	Time        time.Time
	URL         string
	Type        string
	Parent      int
	Descendants int
	Score       int
	Deleted     bool
	Dead        bool
	Poll        int
	Depth       int
}

type ItemRequest struct {
	ID int `form:"id" binding:"required"`
}

type TopRequest struct {
	Limit int `form:"limit"`
}

type BestRequest struct {
	Limit     int       `form:"limit"`
	DateStart time.Time `form:"start" time_format:"2006-01-02" time_utc:"1"`
	DateEnd   time.Time `form:"end" time_format:"2006-01-02" time_utc:"1"`
}

const OneDay = 24 * time.Hour

func main() {
	r := gin.Default()
	db := initDB()
	initTemplate(r)

	store := persistence.NewInMemoryStore(time.Second)

	r.GET("/", cache.CachePage(store, 5*time.Minute, func(c *gin.Context) {
		var request TopRequest
		c.Bind(&request)
		if request.Limit == 0 {
			request.Limit = 100
		}
		var topPosts []TopPost
		db.Raw(`
		SELECT 
			*, 
			(score - 1) / pow((EXTRACT(epoch FROM NOW() - time)/3600)+2, 1.8) AS score_top
		FROM items
		WHERE time > NOW() - interval '7' day AND type = 'story' AND NOT deleted
		ORDER BY score_top DESC NULLS LAST
		LIMIT ?
		`, request.Limit).Find(&topPosts)
		var lastUpdated time.Time
		db.Raw(`SELECT max(time) FROM items;`).Find(&lastUpdated)
		c.HTML(http.StatusOK, "top.tmpl", gin.H{
			"posts":       topPosts,
			"lastUpdated": lastUpdated,
		})
	}))
	r.GET("/item", cache.CachePage(store, 10*time.Minute, func(c *gin.Context) {
		var request ItemRequest
		c.Bind(&request)

		// Fetch parent first to get the time post
		// This optimize the recursive child call to only query specific time chunk
		var parent Item
		db.Table("items").Find(&parent, Item{ID: request.ID})

		var items []Item
		// https://www.postgresql.org/docs/current/queries-with.html#QUERIES-WITH-RECURSIVE
		db.Table("items").Raw(`
		WITH RECURSIVE items_tree AS (
			SELECT id, title, REPLACE("text", 'news.ycombinator.com', 'hn.adhikasp.my.id') "text", "by", time, url, 0 depth
			FROM items
			WHERE id = ? AND time = ?
			
			UNION ALL
			
			SELECT items.id, items.title, REPLACE(items."text", 'news.ycombinator.com', 'hn.adhikasp.my.id') "text", items."by", items.time, items.url, items_tree.depth + 1
			FROM items_tree
			JOIN items ON items.parent = items_tree.id AND items.time > items_tree.time
			WHERE items.time BETWEEN ?::date AND ?::date + interval '1' month -- Optimization assuming no new comment after 1 month
			AND NOT items.deleted
		) SEARCH DEPTH FIRST BY id SET ordercol
		SELECT * FROM items_tree 
		ORDER BY ordercol;
		`, request.ID, parent.Time, parent.Time, parent.Time).Find(&items)
		c.HTML(http.StatusOK, "post.tmpl", gin.H{
			"items":  items,
			"parent": parent,
		})
	}))

	r.GET("/best", cache.CachePage(store, 5*time.Minute, func(c *gin.Context) {
		var request BestRequest
		c.Bind(&request)
		if request.Limit == 0 {
			request.Limit = 100
		}
		if request.DateStart.IsZero() {
			request.DateStart = time.Now().Add(-OneDay).Truncate(OneDay)
		}
		if request.DateEnd.IsZero() || request.DateEnd.Before(request.DateStart) {
			request.DateEnd = request.DateStart.Add(OneDay).Truncate(OneDay)
		}
		var bestPosts []TopPost
		db.Raw(`
		SELECT
			*
		FROM items
		WHERE 
		    time >= ?::date AND time < ?::date + interval '1' day - interval '1' second
		AND type = 'story' 
		AND NOT deleted
		ORDER BY score DESC NULLS LAST
		LIMIT ?;
		`, request.DateStart, request.DateEnd, request.Limit).Find(&bestPosts)
		c.HTML(http.StatusOK, "top.tmpl", gin.H{
			"posts": bestPosts,
			"start": request.DateStart,
			"end":   request.DateEnd,
		})
	}))
	r.Run(":9888")
}

func initDB() *gorm.DB {
	dsn := "host=arjuna user=postgres password=password dbname=hackernews port=5433 sslmode=disable TimeZone=Asia/Jakarta"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("cannot connect to database")
	}
	return db
}

func initTemplate(r *gin.Engine) {
	funcMap := map[string]any{}
	funcMap["unescapeHtml"] = func(s string) template.HTML {
		return template.HTML(s)
	}
	funcMap["multiply"] = func(a int, b int) int {
		return a * b
	}
	funcMap["add"] = func(a int, b int) int {
		return a + b
	}
	r.SetFuncMap(funcMap)
	r.LoadHTMLGlob("templates/*")
}
