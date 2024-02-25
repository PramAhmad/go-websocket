package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
)

var (
	socket        = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(r *http.Request) bool { return true }}
	serversockets = make(map[*websocket.Conn]bool)
	db, _         = sql.Open("mysql", "root:Tasikmalaya123..@tcp(localhost:3306)/chartsocket")
)

func main() {
	db, err := ConnMysql()
	if err != nil {
		log.Fatal("Failed to connect to MySQL:", err)
	}
	defer db.Close()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Next()
	})

	r.GET("/ws", func(c *gin.Context) {
		conn, err := socket.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()
		serversockets[conn] = true
		HandlerWebsocket(conn)
	})

	r.GET("/chart", GetChart)

	r.POST("/chart", PushChart)

	r.Run(":8081")
}

func ConnMysql() (*sql.DB, error) {
	conn, err := sql.Open("mysql", "root:Tasikmalaya123..@tcp(localhost:3306)/chartsocket")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Println(err)
	}
	return conn, nil
}

func HandlerWebsocket(conn *websocket.Conn) {
	// update when data puhsed
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			delete(serversockets, conn)
			break
		}
	}

}

func GetChart(c *gin.Context) {
	var charts []struct {
		IDProduct int `json:"id"`
		Quantity  int `json:"quantity"`
	}

	rows, err := db.Query("SELECT id_produk, quantity FROM chart")
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data from the database"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var chart struct {
			IDProduct int `json:"id"`
			Quantity  int `json:"quantity"`
		}
		if err := rows.Scan(&chart.IDProduct, &chart.Quantity); err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data"})
			return
		}
		charts = append(charts, chart)
	}

	c.JSON(http.StatusOK, charts)
}

func PushChart(c *gin.Context) {
	var body struct {
		IDProduct int `json:"id_product"`
		Quantity  int `json:"quantity"`
	}
	if err := c.Bind(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.Exec("INSERT INTO chart (id_produk, quantity) VALUES (?, ?)", body.IDProduct, body.Quantity)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save to the database"})
		return
	}

	type Chart struct {
		IDProduct int `json:"id"`
		Quantity  int `json:"quantity"`
	}

	res := Chart{
		IDProduct: body.IDProduct,
		Quantity:  body.Quantity,
	}

	for conn := range serversockets {
		if err := conn.WriteJSON([]Chart{res}); err != nil {
			log.Println(err)
			conn.Close()
			delete(serversockets, conn)
		}
	}

	c.JSON(http.StatusOK, res)
}
