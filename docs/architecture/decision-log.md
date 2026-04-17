# Architecture Decision Log: The "Service-Based" Strategy

**Context:** The objective was to determine the optimal organizational structure for the Fleexa backend using Go on AWS Lambda. The decision required balancing **system stability**, **development speed**, and **performance**.

### 1. The Standard Approaches Evaluated

Two common serverless patterns were considered:

- **The Monolith:** Deploying all logic (IoT handling, Auth, API) as a single Lambda function.
- **Micro-Functions:** Deploying a separate, isolated function for every single action (e.g., `login`, `saveTelemetry`, `getHistory`).

### 2. The Challenge: Fleexa's Constraints

Evaluation of the standard approaches revealed specific limitations regarding Fleexa's operational needs:

- **Safety & Isolation:** The "Monolith" pattern creates tight coupling. A deployment error in a non-critical module (e.g., User Profile) risks crashing the critical **IoT Data Pipeline**. Fleexa requires the sensor ingestion layer to remain operational regardless of UI updates.
- **Performance (Cold Starts):** The "Micro-Functions" pattern introduces latency. Rarely invoked functions suffer from "Cold Starts" (delays while AWS provisions the runtime). This results in a slow user experience for the mobile app.
- **Maintenance Overhead:** Managing 20+ separate resources and deployment pipelines creates unnecessary complexity.

### 3. The Solution: Service-Based Architecture
To address these challenges, the backend is organized into a **Service-Based Architecture**. This strategy separates concerns based on the **source of the request** rather than the specific function.

This results in two optimized services:

#### **A. The `iot-ingestion` Service (The Data Plane)**

- **Function:** Listens to **AWS IoT Core** to handle high-speed telemetry streams.
- **Rationale:** By keeping this separate, the data pipeline processes sensor readings and writes to DynamoDB **independently**. This ensures data integrity and system availability regardless of mobile app activity or updates.

#### **B. The `api-service` (The Control Plane)**

- **Function:** Handles all REST API routes for the **Flutter App** (Auth, Dashboard, Commands) into a single Go binary.
- **Rationale:** This aggregates traffic into one Lambda function, keeping the execution environment **"warm"**. This mitigates the Cold Start problem, ensuring a snappy and responsive user experience.

### 4. The Verdict

This architecture provides the **benefits of both approaches**. It secures the **safety isolation** required for critical IoT infrastructure (preventing App errors from crashing the pipeline) while avoiding the **operational complexity** and **latency issues** of managing a large number of micro-functions.
