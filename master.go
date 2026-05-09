package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"
)

// Slave configuration - EDIT THESE FOR YOUR NETWORK
var slaves = map[string]string{
    "slave-1": "http://localhost:8081",
    "slave-2": "http://localhost:8082",
    "slave-3": "http://localhost:8083",
}

type Master struct {
    slaves     map[string]string
    chunks     []string
    results    map[string]int
    resultChan chan map[string]int
    mu         sync.Mutex
}

// CharStats struct for character statistics
type CharStats struct {
    Name       string
    Count      int
    Importance string
}

func NewMaster() *Master {
    return &Master{
        slaves:     slaves,
        chunks:     []string{},
        results:    make(map[string]int),
        resultChan: make(chan map[string]int, 100),
    }
}

// Split novel into chunks
func (m *Master) splitFile(filename string, chunkSize int) error {
    data, err := os.ReadFile(filename)
    if err != nil {
        return err
    }

    content := string(data)
    words := strings.Fields(content)

    // Create chunks of approximately chunkSize words
    for i := 0; i < len(words); i += chunkSize {
        end := i + chunkSize
        if end > len(words) {
            end = len(words)
        }
        chunk := strings.Join(words[i:end], " ")
        m.chunks = append(m.chunks, chunk)
    }

    fmt.Printf("📚 Split novel into %d chunks\n", len(m.chunks))
    return nil
}

// Send map task to a slave
func (m *Master) sendMapTask(slaveURL string, chunkID int, chunkData string, characters []string) (map[string]int, error) {
    task := map[string]interface{}{
        "task_id":    fmt.Sprintf("task-%d", chunkID),
        "chunk_id":   chunkID,
        "data":       chunkData,
        "characters": characters,
    }

    jsonData, err := json.Marshal(task)
    if err != nil {
        return nil, err
    }

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Post(slaveURL+"/analyze", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        TaskID  string         `json:"task_id"`
        ChunkID int            `json:"chunk_id"`
        Counts  map[string]int `json:"counts"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    return result.Counts, nil
}

// Distribute work to all slaves using goroutines
func (m *Master) distributeWork(characters []string) {
    var wg sync.WaitGroup

    // Convert slaves map to slice for round-robin
    slaveList := make([]string, 0, len(m.slaves))
    for _, url := range m.slaves {
        slaveList = append(slaveList, url)
    }

    // Launch a goroutine for each chunk
    for i, chunk := range m.chunks {
        wg.Add(1)
        go func(chunkID int, chunkData string) {
            defer wg.Done()

            // Round-robin slave selection
            slaveURL := slaveList[chunkID%len(slaveList)]

            fmt.Printf("📤 Sending chunk %d to %s\n", chunkID, slaveURL)

            counts, err := m.sendMapTask(slaveURL, chunkID, chunkData, characters)
            if err != nil {
                fmt.Printf("❌ Error processing chunk %d: %v\n", chunkID, err)
                return
            }

            m.resultChan <- counts
            fmt.Printf("✅ Chunk %d completed\n", chunkID)
        }(i, chunk)
    }

    // Close channel when all goroutines are done
    go func() {
        wg.Wait()
        close(m.resultChan)
    }()
}

// Collect and aggregate results from channel
func (m *Master) collectResults() {
    for counts := range m.resultChan {
        m.mu.Lock()
        for character, count := range counts {
            m.results[character] += count
        }
        m.mu.Unlock()
    }
}

// Calculate importance and generate report
func (m *Master) generateReport(characters []string) {
    fmt.Println("\n" + strings.Repeat("=", 60))
    fmt.Println("📊 CHARACTER IMPORTANCE REPORT")
    fmt.Println(strings.Repeat("=", 60))

    var stats []CharStats
    totalMentions := 0

    // Calculate total mentions
    for _, count := range m.results {
        totalMentions += count
    }

    // Create stats for each character
    for _, char := range characters {
        count := m.results[char]

        importance := ""
        switch {
        case count > 100:
            importance = "⭐ MAIN PROTAGONIST"
        case count > 50:
            importance = "🌟 SUPPORTING LEAD"
        case count > 20:
            importance = "📖 SIGNIFICANT ROLE"
        case count > 5:
            importance = "👤 MINOR CHARACTER"
        case count > 0:
            importance = "💭 BRIEFLY MENTIONED"
        default:
            importance = "❌ NOT FOUND"
        }

        stats = append(stats, CharStats{char, count, importance})
    }

    // Sort by count (bubble sort)
    for i := 0; i < len(stats)-1; i++ {
        for j := i + 1; j < len(stats); j++ {
            if stats[j].Count > stats[i].Count {
                stats[i], stats[j] = stats[j], stats[i]
            }
        }
    }

    // Print report
    for i, stat := range stats {
        fmt.Printf("\n%d. %s\n", i+1, stat.Name)
        fmt.Printf("   📊 Appearances: %d times\n", stat.Count)
        if totalMentions > 0 {
            fmt.Printf("   📈 Percentage: %.1f%%\n", float64(stat.Count)/float64(totalMentions)*100)
        }
        fmt.Printf("   🎭 Role: %s\n", stat.Importance)

        // Visual bar
        barLength := stat.Count / 5
        if barLength > 50 {
            barLength = 50
        }
        if barLength > 0 {
            bar := strings.Repeat("█", barLength)
            fmt.Printf("   %s\n", bar)
        }
    }

    // Save to file
    m.saveResults(stats)
}

// Save results to file
func (m *Master) saveResults(stats []CharStats) {
    file, err := os.Create("character_importance.txt")
    if err != nil {
        fmt.Printf("❌ Error saving results: %v\n", err)
        return
    }
    defer file.Close()

    fmt.Fprintf(file, "CHARACTER IMPORTANCE ANALYSIS\n")
    fmt.Fprintf(file, "=============================\n\n")

    for i, stat := range stats {
        fmt.Fprintf(file, "%d. %s\n", i+1, stat.Name)
        fmt.Fprintf(file, "   Appearances: %d\n", stat.Count)
        fmt.Fprintf(file, "   Role: %s\n\n", stat.Importance)
    }

    fmt.Printf("\n💾 Results saved to character_importance.txt\n")
}

func main() {
    // Check command line arguments
    if len(os.Args) < 3 {
        fmt.Println("Usage: go run master.go <novel_file> <character1> <character2> ...")
        fmt.Println("Example: go run master.go novel.txt Harry Ron Hermione")
        fmt.Println("\nConfigure slave IPs in the 'slaves' map at the top of this file")
        return
    }

    novelFile := os.Args[1]
    characters := os.Args[2:]

    fmt.Println(strings.Repeat("=", 60))
    fmt.Println("🎭 DISTRIBUTED CHARACTER ANALYZER")
    fmt.Println(strings.Repeat("=", 60))
    fmt.Printf("📖 Novel: %s\n", novelFile)
    fmt.Printf("👥 Characters: %v\n", characters)
    fmt.Printf("🖥️  Slaves: %d configured\n", len(slaves))

    for id, url := range slaves {
        fmt.Printf("   - %s: %s\n", id, url)
    }

    master := NewMaster()

    // Split the novel
    if err := master.splitFile(novelFile, 1000); err != nil {
        fmt.Printf("❌ Error: %v\n", err)
        return
    }

    // Start processing
    fmt.Println("\n🚀 Starting distributed analysis...")
    startTime := time.Now()

    master.distributeWork(characters)
    master.collectResults()

    duration := time.Since(startTime)
    fmt.Printf("\n⏱️  Processing completed in %v\n", duration)

    // Generate report
    master.generateReport(characters)
}