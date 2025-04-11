This Go application is designed to interact with Azure Cosmos DB, specifically to read changes from a Cosmos DB container using the **Change Feed** feature. It also includes functionality to add test items to the Cosmos DB container.

### **Key Features of the Application**
1. **Change Feed Reader**:
   - The application reads changes from a Cosmos DB container using the Change Feed API.
   - It retrieves documents that have been added, updated, or deleted since the last read operation.
   - The `readChangeFeed` function handles the logic for reading the Change Feed, with retry logic for transient errors like `401 Unauthorized`.

2. **Test Item Creator**:
   - The `addTestItem` function adds a new test item to the Cosmos DB container. This is useful for testing the Change Feed functionality.

3. **Custom HTTP Handler**:
   - The application exposes an HTTP endpoint (`/api/ChangeFeedTrigger`) to trigger the Change Feed reading process.
   - This is implemented using Go's `http` package.

4. **Authentication**:
   - The application uses a Cosmos DB account key for authentication, which is used to generate an authorization token for API requests.

5. **Retry Logic**:
   - The `readChangeFeed` function includes retry logic to handle transient errors, such as `401 Unauthorized` or network issues.

---

### **How to Configure and Run the Application**

#### **1. Prerequisites**
   - Install Go on your machine: [Download Go](https://go.dev/dl/).
   - Install Azure Functions Core Tools: [Azure Functions Core Tools](https://learn.microsoft.com/en-us/azure/azure-functions/functions-run-local).
   - Set up an Azure Cosmos DB account with a database and container:
     - Create a Cosmos DB account in the Azure portal.
     - Create a database (e.g., `veeamDatabase`).
     - Create a container (e.g., `veeamdata`) with a partition key (e.g., `/id`).

#### **2. Configure Environment Variables**
   - Create a `.env` file in the root directory of your project with the following content:
     ```plaintext
     COSMOSDB_ENDPOINT=https://<your-cosmosdb-account>.documents.azure.com:443/
     COSMOSDB_KEY=<your-cosmosdb-account-key>
     ```
   - Replace `<your-cosmosdb-account>` and `<your-cosmosdb-account-key>` with your Cosmos DB account's endpoint and key.

#### **3. Update the Code**
   - Ensure the `endpoint` and `key` variables in the `changeFeedTriggerHandler` function are set to read from the `.env` file:
     ```go
     endpoint := os.Getenv("COSMOSDB_ENDPOINT")
     key := os.Getenv("COSMOSDB_KEY")
     ```

#### **4. Install Dependencies**
   - Run the following command to install the required Go dependencies:
     ```bash
     go mod tidy
     ```

#### **5. Run the Application**
   - Start the application using:
     ```bash
     go run main.go
     ```
   - The application will start an HTTP server on port `8080` (or the port specified in the `FUNCTIONS_CUSTOMHANDLER_PORT` environment variable).

#### **6. Test the Application**
   - Use a tool like `curl` or Postman to send a request to the HTTP endpoint:
     ```bash
     curl http://localhost:8080/api/ChangeFeedTrigger
     ```
   - The application will:
     - Read changes from the Cosmos DB container.
     - Log the changes to the console.
     - Add a test item to the container.

#### **7. Deploy to Azure**
   - To deploy the application to Azure Functions:
     1. Create an Azure Function App in the Azure portal.
     2. Configure the Function App to use the **Custom Handler** runtime:
        - Set the `FUNCTIONS_WORKER_RUNTIME` app setting to `custom`.
        - Set the `FUNCTIONS_CUSTOMHANDLER_PORT` app setting to `8080` (or the port used by your application).
     3. Deploy the application using the Azure CLI:
        ```bash
        func azure functionapp publish <your-function-app-name>
        ```

---

### **How the Application Works**
1. **Change Feed Reading**:
   - The `readChangeFeed` function sends a `GET` request to the Cosmos DB Change Feed API endpoint.
   - It retrieves documents that have changed since the last read operation, using the `continuationToken` to track progress.

2. **Authorization**:
   - The `generateAuthToken` function generates an HMAC-based authorization token using the Cosmos DB account key.
   - This token is included in the `Authorization` header of each API request.

3. **Retry Logic**:
   - If a transient error (e.g., `401 Unauthorized`) occurs, the application retries the request with an exponential backoff.

4. **Adding Test Items**:
   - The `addTestItem` function creates a new document in the Cosmos DB container with a unique ID and some test data.

---

### **Best Practices**
1. **Secure Secrets**:
   - Use Azure Key Vault or environment variables to securely store the Cosmos DB account key.

2. **Error Handling**:
   - Improve error handling to gracefully handle non-transient errors and provide meaningful error messages.

3. **Scalability**:
   - Use Azure Functions' built-in scaling capabilities to handle high volumes of Change Feed data.

4. **Logging**:
   - Use Azure Application Insights for centralized logging and monitoring.

5. **Optimize Change Feed Processing**:
   - Use the `continuationToken` to ensure that no changes are missed between function executions.

By following these steps, you can configure and run the application locally or deploy it to Azure for production use.