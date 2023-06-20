package main

import (
	"SCUBA/models"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var (
	db  *sql.DB
	err error
)

func initDb() error {

	db, err = sql.Open("mysql", "root:test123@tcp(localhost:3306)/scuba_logs")
	if err != nil {
		return err
	}

	// Test the database connection
	err = db.Ping()
	if err != nil {
		return err
	}

	return nil
}

func createDiver(g *gin.Context) error {
	var diver models.Diver

	if err := g.ShouldBindJSON(&diver); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return err
	}

	ins, err := db.Prepare("Insert Into divers (name,diverEq) Values (?,?)")
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return err
	}
	defer ins.Close()

	_, err = ins.Exec(diver.Name, string(diver.DiverEqp))
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create diver profile"})
		return err
	}

	g.Status(http.StatusCreated)
	return nil
}

func logNewDive(g *gin.Context) error {
	var divelog models.DiveLog

	if err := g.ShouldBindJSON(&divelog); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create diver profile"})
		return err
	}
	ins, err := db.Prepare("Insert Into divelog (diverId,depth,timestamp) values (?,?,?)")
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create diver profile"})
		return err
	}
	ins.Close()
	_, err = ins.Exec(divelog.DiverId, divelog.Depth, time.Now())
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log new dive"})
		return err
	}
	g.Status(http.StatusAccepted)
	return nil
}

func getAllDivers(g *gin.Context) error {
	var diversLogs []models.DiveLog

	nameOrID := g.Query("nameOrId")

	diver, err := db.Query("SELECT id, diverId, depth, timestamp FROM divelogs WHERE diverId = ? OR diverId IN (SELECT id FROM divers WHERE name = ?)", nameOrID, nameOrID)
	if err != nil {
		return err
	}
	defer diver.Close()

	for diver.Next() {
		var diveLog models.DiveLog
		err := diver.Scan(&diveLog.Id, &diveLog.DiverId, &diveLog.Depth, &diveLog.Timestamp)
		if err != nil {
			g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve diver's dive log"})
			return err
		}
		diversLogs = append(diversLogs, diveLog)
	}

	g.JSON(http.StatusOK, diversLogs)
	return nil
}
func getMaxDepth(g *gin.Context) {
	nameOrId := g.Query("nameOrId")

}
func main() {
	// Setup database connection
	err := initDb()
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}
	router := gin.Default()
	router.Run(":8080")
}
