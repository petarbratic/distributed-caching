# Distributed Caching System

This project is a simple distributed caching system used to test different caching techniques and their impact on backend performance.

---

## System Architecture

The system consists of the following components:

*   **Load Balancer:** Nginx routing traffic to the gateways.
*   **Gateways:** Two gateway instances behind the load balancer.
*   **L1 Cache:** Fast, in-memory cache local to each gateway.
*   **L2 Cache:** Shared distributed cache using **Redis**.
*   **Backend Service:** Mock service with configurable latency and request limits.
*   **Frontend:** Angular dashboard for real-time system configuration.
*   **Monitoring:** Prometheus and Grafana for metrics visualization.
*   **Load Testing:** k6 scripts tailored for different load scenarios.

---

## Supported Caching Features

### Core Mechanics
*   **Configurable Size & TTL:** Easily adjust cache capacity and time-to-live.
*   **Backend Timeout:** Configurable limits to prevent gateway hanging.

### Anti-stampede strategies
*   **SingleFlight:** Request coalescing to prevent backend thundering herds.
*   **Distributed Locking:** Implemented using Redis to synchronize gateway instances.
*   **Probabilistic Early Refresh:** Refreshes popular keys before they expire.
*   **Adaptive TTL:** Automatically adjusts expiration based on current backend load.


---

## Tech Stack

| Component | Technology |
| :--- | :--- |
| **Load Balancer** | Nginx |
| **Shared Cache** | Redis |
| **Frontend** | Angular |
| **Monitoring** | Prometheus & Grafana |
| **Load Testing** | k6 |
| **Orchestration** | Docker Compose |
