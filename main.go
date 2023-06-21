package main

import (
	"SCUBA/models"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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

	err = db.Ping()
	if err != nil {
		return err
	}

	return nil
}

func createDiver(g *gin.Context) {
	var diver models.Diver

	if err := g.ShouldBindJSON(&diver); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ins, err := db.Prepare("Insert Into divers (name,diverEqp) Values (?,?)")
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	defer ins.Close()

	_, err = ins.Exec(diver.Name, string(diver.DiverEqp))
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create diver profile"})
		return
	}

	g.Status(http.StatusCreated)
	return
}

func addNewDive(g *gin.Context) {
	var diveLog models.DiveLog

	if err := g.ShouldBindJSON(&diveLog); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create diver profile"})
		return
	}

	if diveLog.Depth > 40 {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Maximum allowed depth exceeded"})
		return
	}

	var diveCount int
	err = db.QueryRow("SELECT COUNT(*) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&diveCount)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dive count"})
		return
	}

	var diveTimes int

	err = db.QueryRow("SELECT COUNT(*) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&diveTimes)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dive count"})
		return
	}

	if diveTimes >= 10 {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Maximum allowed dives per diver reached"})
		return
	}

	var lastInterval int
	err = db.QueryRow("SELECT IFNULL(MAX(interval), 0) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&lastInterval)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve previous interval"})
		return
	}

	ins, err := db.Prepare("Insert Into divelog (diverId,depth,timestamp) values (?,?,?)")
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create diver profile"})
		return
	}
	ins.Close()
	_, err = ins.Exec(diveLog.DiverId, diveLog.Depth, time.Now())
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log new dive"})
		return
	}
	g.Status(http.StatusAccepted)
	return
}

func getAllDivers(g *gin.Context) {
	var diversLogs []models.DiveLog

	nameOrID := g.Query("nameOrId")

	diver, err := db.Query("SELECT id, diverId, depth, timestamp FROM divelogs WHERE diverId = ? OR diverId IN (SELECT id FROM divers WHERE name = ?)", nameOrID, nameOrID)
	if err != nil {
		return
	}
	defer diver.Close()

	for diver.Next() {
		var diveLog models.DiveLog
		err := diver.Scan(&diveLog.Id, &diveLog.DiverId, &diveLog.Depth, &diveLog.Timestamp)
		if err != nil {
			g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve diver's dive log"})
			return
		}
		diversLogs = append(diversLogs, diveLog)
	}

	g.JSON(http.StatusOK, diversLogs)
	return
}
func getMaxDepth(g *gin.Context) {
	nameOrId := g.Query("nameOrId")

	maxDepth, err := db.Query("SELECT MAX(depth) FROM divelogs WHERE diverId = ? OR diverId IN (SELECT id FROM divers WHERE name = ?)", nameOrId, nameOrId)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query maximum depth"})
		return
	}
	g.JSON(http.StatusOK, gin.H{"maxDepth": maxDepth})
	return
}

func queryDiversInformation(c *gin.Context) {
	diverIDs := c.Query("diverIds")

	// Parse the diver IDs
	idRanges := parseDiverIDs(diverIDs)
	if len(idRanges) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid diver IDs"})
		return
	}

	// Retrieve the diver information from the database
	var divers []models.Diver
	for _, idRange := range idRanges {
		query := buildQuery(idRange)
		rows, err := db.Query(query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query divers information"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var diver models.Diver
			err := rows.Scan(&diver.Id, &diver.Name, &diver.DiverEqp)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query divers information"})
				return
			}
			divers = append(divers, diver)
		}
	}

	c.JSON(http.StatusOK, divers)
}

// parseDiverIDs parses the comma-delimited diver IDs or ranges.
func parseDiverIDs(diverIDs string) []IDRange {
	var idRanges []IDRange

	ids := splitIDs(diverIDs)
	for _, id := range ids {
		if id == "" {
			continue
		}

		if rangeID := parseIDRange(id); rangeID != nil {
			idRanges = append(idRanges, *rangeID)
		}
	}

	return idRanges
}

// splitIDs splits the comma-delimited IDs.
func splitIDs(diverIDs string) []string {
	return strings.Split(diverIDs, ",")
}

// parseIDRange parses the ID range.
func parseIDRange(idRange string) *IDRange {
	parts := strings.Split(idRange, "-")
	if len(parts) != 2 {
		return nil
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}

	return &IDRange{Start: start, End: end}
}

// buildQuery builds the SQL query for retrieving divers information by ID range.
func buildQuery(idRange IDRange) string {
	query := "SELECT id, name, diverEqp FROM divers WHERE id >= ? AND id <= ?"
	return fmt.Sprintf(query, idRange.Start, idRange.End)
}

// IDRange represents a range of IDs.
type IDRange struct {
	Start int
	End   int
}

func generateDiveReport(g *gin.Context) {
	// Retrieve the total number of dives per diver from the database
	rows, err := db.Query("SELECT diverId, COUNT(*) as totalDives FROM divelogs GROUP BY diverId")
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate dive report"})
		return
	}
	defer rows.Close()

	diveReport := make(map[int]int)
	for rows.Next() {
		var diverID, totalDives int
		err := rows.Scan(&diverID, &totalDives)
		if err != nil {
			g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate dive report"})
			return
		}
		diveReport[diverID] = totalDives
	}

	g.JSON(http.StatusOK, diveReport)
}
func main() {
	// Setup database connection
	err := initDb()
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	// Create Gin router
	router := gin.Default()

	// Define backend routes
	router.POST("/divers", createDiver)
	router.POST("/dives", addNewDive)
	router.GET("/dives", getAllDivers)
	router.GET("/dives/report", generateDiveReport)
	router.GET("/dives/maxdepth", getMaxDepth)
	router.GET("/divers", queryDiversInformation)

	// Run the server
	router.Run(":8080")
}
