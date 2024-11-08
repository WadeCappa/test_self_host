package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	xss "github.com/sahilchopra/gin-gonic-xss-middleware"
)

type messagePostRequest struct {
	Content string `form:"content" json:"content" xml:"content" binding:"required"`
}

type message struct {
	Content    string
	TimePosted time.Time
}

func runWithDb(consumer func(*sql.DB)) {
	host := "db"
	port := 5432
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		fmt.Println(err)
	}
	consumer(db)
}

func asMessages(messages *sql.Rows) []message {
	var res []message
	for messages.Next() {
		var content string
		var timePosted int64
		if err := messages.Scan(&content, &timePosted); err == nil {
			res = append(res, message{
				content,
				time.Unix(0, timePosted*int64(time.Millisecond)),
			})
		} else {
			fmt.Println(err)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].TimePosted.After(res[j].TimePosted)
	})
	return res
}

func getMessages(conn *sql.DB) []message {
	res, err := conn.Query("select content, time_posted from user_post")
	defer res.Close()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res)
	return asMessages(res)
}

func main() {
	r := gin.Default()
	var xssMdlwr xss.XssMw
	r.Use(xssMdlwr.RemoveXss())
	r.SetTrustedProxies(nil)

	mode := os.Getenv("MODE")
	fmt.Println(mode)
	config := cors.DefaultConfig()
	config.AllowMethods = []string{"GET", "POST"}
	config.AllowOriginFunc = func(origin string) bool {
		fmt.Println(origin)
		switch mode {
		case "production":
			return origin == "https://cantseewater.online"
		case "local":
			return origin == "http://localhost:3000"
		default:
			fmt.Println(fmt.Errorf("did not recognize the deployement mode: %s", mode))
			return false
		}
	}
	r.Use(cors.New(config))

	r.POST("/write", func(c *gin.Context) {
		var messagePost messagePostRequest
		if err := c.ShouldBindJSON(&messagePost); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		timePosted := time.Now()
		fmt.Println(messagePost.Content)

		runWithDb(func(conn *sql.DB) {
			res, err := conn.Query("insert into user_post (time_posted, content) values ($1, $2)", timePosted.UnixMilli(), messagePost.Content)
			fmt.Println(res)
			fmt.Println(err)
			c.JSON(http.StatusOK, getMessages(conn))
		})
	})

	r.GET("/get", func(c *gin.Context) {
		runWithDb(func(conn *sql.DB) {
			c.JSON(http.StatusOK, getMessages(conn))
		})
	})

	r.Run()
}
