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
)s

func initDb() error {

	db, err = sql.Open("mysql", "root:test123@tcp(localhost:3306)/scuba_logs")
	if err != nil {
		return err
	}

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
	var diveLog models.DiveLog

	if err := g.ShouldBindJSON(&diveLog); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create diver profile"})
		return err
	}

	if diveLog.Depth > 40 {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Maximum allowed depth exceeded"})
		return err
	}

	var diveCount int
	err = db.QueryRow("SELECT COUNT(*) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&diveCount)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dive count"})
		return err
	}

	var diveTimes int

	err = db.QueryRow("SELECT COUNT(*) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&diveTimes)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dive count"})
		return err
	}

	if diveTimes >= 10 {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Maximum allowed dives per diver reached"})
		return err
	}

	ins, err := db.Prepare("Insert Into divelog (diverId,depth,timestamp) values (?,?,?)")
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create diver profile"})
		return err
	}
	ins.Close()
	_, err = ins.Exec(diveLog.DiverId, diveLog.Depth, time.Now())
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
func getMaxDepth(g *gin.Context) error {
	nameOrId := g.Query("nameOrId")

	maxDepth, err := db.Query("SELECT MAX(depth) FROM divelogs WHERE diverId = ? OR diverId IN (SELECT id FROM divers WHERE name = ?)", nameOrId, nameOrId)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query maximum depth"})
		return err
	}
	g.JSON(http.StatusOK, gin.H{"maxDepth": maxDepth})
	return nil
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
