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

	db, err = sql.Open("mysql", "root:test123@tcp(localhost:3306)/scuba_logs?parseTime=true")
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

	// Bind the request body to the DiveLog struct
	if err := g.ShouldBindJSON(&diveLog); err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Check if the depth exceeds the maximum allowed depth
	if diveLog.Depth > 40 {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Maximum allowed depth exceeded"})
		return
	}

	// Check the dive count for the diver
	var diveCount int
	err := db.QueryRow("SELECT COUNT(*) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&diveCount)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dive count"})
		return
	}

	// Check if the maximum allowed dives per diver is reached
	if diveCount >= 10 {
		g.JSON(http.StatusBadRequest, gin.H{"error": "Maximum allowed dives per diver reached"})
		return
	}

	// Check if it's the first dive for the diver
	var isFirstDive int
	err = db.QueryRow("SELECT COUNT(*) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&isFirstDive)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check if it's the first dive"})
		return
	}

	// Calculate the minimum interval between dives
	var minInterval int
	if isFirstDive == 0 {

		minInterval = 0
	} else {
		// Retrieve the last interval from the previous dive
		var lastInterval int
		err = db.QueryRow("SELECT TIMESTAMPDIFF(MINUTE, MAX(timestamp), NOW()) FROM divelogs WHERE diverId = ?", diveLog.DiverId).Scan(&lastInterval)
		if err != nil {
			g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve the previous interval"})
			return
		}

		// Calculate the minimum interval based on the depth
		minInterval = lastInterval + int(diveLog.Depth*2)
	}

	// Insert the dive log into the database
	stmt, err := db.Prepare("INSERT INTO divelogs (diverId, depth, timestamp) VALUES (?, ?, ?)")
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare the dive log insertion"})
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(diveLog.DiverId, diveLog.Depth, time.Now())
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log the new dive"})
		return
	}

	g.JSON(http.StatusAccepted, gin.H{
		"message":              "Dive logged successfully",
		"minimum_allowed_time": minInterval,
	})
}

func getAllDivers(g *gin.Context) {
	var diversLogs []models.DiveLog

	nameOrID := g.Query("nameOrId")

	diver, err := db.Query("SELECT id, diverId, depth, timestamp FROM divelogs WHERE diverId = ? OR diverId IN (SELECT id FROM divers WHERE name = ?)", nameOrID, nameOrID)
	if err != nil {
		g.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve dive logs"})

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

	query := "SELECT MAX(depth) FROM divelogs WHERE diverId = ?"
	rows, err := db.Query(query, nameOrId)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var maxDepth sql.NullFloat64
	if rows.Next() {
		err = rows.Scan(&maxDepth)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Check if the maximum depth is NULL, and set it to 0.0 if it is
	if maxDepth.Valid {
		fmt.Printf("Maximum depth for diverId %s: %.2f\n", nameOrId, maxDepth.Float64)
	} else {
		fmt.Printf("No maximum depth found for diverId %s\n", nameOrId)
	}
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
