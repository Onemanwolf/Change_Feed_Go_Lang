package main

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    //"net/url"
    //"os"
    "strings"
    "time"

    "github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
    "github.com/joho/godotenv"
)

type Item struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Value int    `json:"value"`
}

type ChangeFeedResponse struct {
    Documents         []Item `json:"Documents"`
    ContinuationToken string `json:"_continuation"`
}

var client = &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
    },
}

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatalf("Error loading .env file")
    }

    customHandlerPort:= "8080" //os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
    //if exists {
    //    fmt.Println("FUNCTIONS_CUSTOMHANDLER_PORT: " + customHandlerPort)
   // }

    mux := http.NewServeMux()

    mux.HandleFunc("/api/ChangeFeedTrigger", changeFeedTriggerHandler) // New handler for change feed

    fmt.Println("Go server Listening...on FUNCTIONS_CUSTOMHANDLER_PORT:", customHandlerPort)
    log.Fatal(http.ListenAndServe(":"+customHandlerPort, mux))
}

func changeFeedTriggerHandler(w http.ResponseWriter, r *http.Request) {
     endpoint := ""  //"https://<your-Azure-Cosmos-DB-Account-name>-eastus.documents.azure.com:443/" //os.Getenv("COSMOSDB_ENDPOINT")
     key :=   "" //os.Getenv("COSMOSDB_KEY")
    databaseName := "veeamDatabase"
    containerName := "veeamdata"



    cred, err := azcosmos.NewKeyCredential(key)
    if err != nil {
        log.Fatalf("Failed to create credential: %v", err)
    }

    cosmosClient, err := azcosmos.NewClientWithKey(endpoint, cred, nil)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    database, err := cosmosClient.NewDatabase(databaseName)
    if err != nil {
        log.Fatalf("Failed to get database: %v", err)
    }

    container, err := database.NewContainer(containerName)
    if err != nil {
        log.Fatalf("Failed to get container: %v", err)
    }

    var continuationToken string
    for {
        changes, newToken, err := readChangeFeed(endpoint, key, databaseName, containerName, continuationToken)
        if err != nil {
            log.Printf("Error reading Change Feed: %v", err)
        } else {
            if len(changes) > 0 {
                log.Printf("Found %d changes", len(changes))
                for _, item := range changes {
                    fmt.Printf("Changed document - ID: %s, Name: %s, Value: %d\n", item.ID, item.Name, item.Value)
                }
            } else {
                log.Printf("No changes detected in this poll")
            }
            continuationToken = newToken
        }

        addTestItem(container)
        log.Printf("Waiting for changes...")
        time.Sleep(1 * time.Second)
    }
}

func readChangeFeed(endpoint, key, databaseID, containerID, continuationToken string) ([]Item, string, error) {
    const maxRetries = 5
    for attempt := 0; attempt < maxRetries; attempt++ {
        startTime := time.Now()
        changes, newToken, err := attemptReadChangeFeed(endpoint, key, databaseID, containerID, continuationToken)
        duration := time.Since(startTime)
        if err == nil {
            log.Printf("Change Feed request succeeded after %d ms", duration.Milliseconds())
            return changes, newToken, nil
        }
        if strings.Contains(err.Error(), "status 401") {
            log.Printf("Retry attempt %d of %d after 401 error (took %d ms): %v", attempt+1, maxRetries, duration.Milliseconds(), err)
            time.Sleep(time.Duration(attempt+1) * time.Second) // Increased backoff
            continue
        }
        return nil, "", err
    }
    log.Printf("All retries failed, resetting continuation token")
    return nil, "", nil
}

func attemptReadChangeFeed(endpoint, key, databaseID, containerID, continuationToken string) ([]Item, string, error) {
    url := fmt.Sprintf("%sdbs/%s/colls/%s/docs", endpoint, databaseID, containerID)
    log.Printf("Request URL: %s", url)

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, "", fmt.Errorf("failed to create request: %v", err)
    }

    date := time.Now().UTC().Format(time.RFC1123)
    date = strings.Replace(date, "UTC", "GMT", 1)
    req.Header.Set("x-ms-date", date)
    req.Header.Set("x-ms-version", "2018-12-31")
    req.Header.Set("A-IM", "Incremental feed")
    if continuationToken != "" {
        req.Header.Set("If-None-Match", continuationToken)
    }

    authToken, err := generateAuthToken("GET", "docs", fmt.Sprintf("dbs/%s/colls/%s", databaseID, containerID), date, key)
    if err != nil {
        return nil, "", fmt.Errorf("failed to generate auth token: %v", err)
    }
    req.Header.Set("Authorization", authToken)

    log.Printf("Request Headers: %+v", req.Header)

    resp, err := client.Do(req)
    if err != nil {
        return nil, "", fmt.Errorf("failed to execute request: %v", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, "", fmt.Errorf("failed to read response body: %v", err)
    }
    log.Printf("Response Status: %d", resp.StatusCode)
    log.Printf("Response Body: %s", string(body))

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
        return nil, "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
    }

    if resp.StatusCode == http.StatusNotModified {
        return nil, continuationToken, nil
    }

    var changeFeed ChangeFeedResponse
    if err := json.Unmarshal(body, &changeFeed); err != nil {
        return nil, "", fmt.Errorf("failed to unmarshal response: %v", err)
    }

    newContinuation := resp.Header.Get("ETag")
    if newContinuation == "" {
        newContinuation = continuationToken
    }

    return changeFeed.Documents, newContinuation, nil
}

func generateAuthToken(verb, resourceType, resourceID, date, key string) (string, error) {
    masterKey, err := base64.StdEncoding.DecodeString(key)
    if err != nil {
        return "", fmt.Errorf("failed to decode master key: %v", err)
    }

    stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s\n\n",
        strings.ToLower(verb),
        strings.ToLower(resourceType),
        resourceID,
        strings.ToLower(date),
    )
    log.Printf("String to Sign: %q", stringToSign)

    h := hmac.New(sha256.New, masterKey)
    _, err = h.Write([]byte(stringToSign))
    if err != nil {
        return "", fmt.Errorf("failed to write HMAC: %v", err)
    }
    signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

    auth := fmt.Sprintf("type=master&ver=1.0&sig=%s", signature)
    return urlEncode(auth), nil
}

func urlEncode(s string) string {
    return strings.ReplaceAll(strings.ReplaceAll(s, "+", "%2B"), " ", "%20")
}

func addTestItem(container *azcosmos.ContainerClient) {
    testItem := Item{
        ID:    fmt.Sprintf("test%d", time.Now().Unix()),
        Name:  "Test Item",
        Value: 100,
    }
    itemData, _ := json.Marshal(testItem)
    pk := azcosmos.NewPartitionKeyString(testItem.ID)
    _, err := container.CreateItem(context.Background(), pk, itemData, nil)
    if err != nil {
        log.Printf("Failed to add test item: %v", err)
    } else {
        log.Printf("Added test item: %s", testItem.ID)
    }
}

// Existing handlers and functions...

