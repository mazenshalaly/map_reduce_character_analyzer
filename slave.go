package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
)

type Slave struct {
    port string
}

func NewSlave(port string) *Slave {
    return &Slave{port: port}
}

// Analyze endpoint - counts character mentions
func (s *Slave) analyzeHandler(w http.ResponseWriter, r *http.Request) {
    var task struct {
        TaskID     string   `json:"task_id"`
        ChunkID    int      `json:"chunk_id"`
        Data       string   `json:"data"`
        Characters []string `json:"characters"`
    }

    if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Convert to lowercase for case-insensitive matching
    text := strings.ToLower(task.Data)
    counts := make(map[string]int)

    // Count each character
    for _, character := range task.Characters {
        charLower := strings.ToLower(character)
        count := 0
        
        // Split into words for better accuracy
        words := strings.Fields(text)
        for _, word := range words {
            // Clean word of punctuation
            word = strings.Trim(word, ".,!?;:()\"'")
            if word == charLower {
                count++
            }
        }
        
        // Also count substrings (for cases like "Harry's")
        if count == 0 {
            count = strings.Count(text, charLower)
        }
        
        counts[character] = count
    }

    result := map[string]interface{}{
        "task_id":  task.TaskID,
        "chunk_id": task.ChunkID,
        "counts":   counts,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)

    fmt.Printf("📊 Processed chunk %d: found %d character mentions\n", 
        task.ChunkID, len(counts))
}

// Health check endpoint
func (s *Slave) healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func main1() {
    // Get port from command line
    if len(os.Args) < 2 {
        fmt.Println("Usage: go run slave.go <port>")
        fmt.Println("Example: go run slave.go 8081")
        return
    }

    port := os.Args[1]
    slave := NewSlave(port)

    // Setup routes
    http.HandleFunc("/analyze", slave.analyzeHandler)
    http.HandleFunc("/health", slave.healthHandler)

    // Start server
    addr := ":" + port
    fmt.Printf("🚀 Slave starting on port %s\n", port)
    fmt.Printf("   Endpoint: http://localhost:%s/analyze\n", port)
    
    if err := http.ListenAndServe(addr, nil); err != nil {
        fmt.Printf("❌ Slave error: %v\n", err)
    }
}