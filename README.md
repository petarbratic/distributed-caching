# Distributed Caching System

An experimental platform for evaluating distributed caching strategies under high and variable load.

The platform is designed to compare the effects of different anti-stampede mechanisms on cache hit ratio, backend load amplification, response latency, and backend stability.

The repository includes the source code, load-generation scripts, monitoring configuration, collected experiment results, and generated comparison charts.

## Content

- [System Overview](#system-overview)
- [Architecture](#architecture)
- [Supported Caching Strategies](#supported-caching-strategies)
- [Repository Structure](#repository-structure)
- [Prerequisites](#prerequisites)
- [Starting the Platform](#starting-the-platform)
- [Service URLs](#service-urls)
- [Configuring the Platform](#configuring-the-platform)
- [Experiment Configurations](#experiment-configurations)
- [Load Test Scenarios](#load-test-scenarios)
- [Running an Experiment](#running-an-experiment)
- [Experiment Results](#experiment-results)
- [Resetting Prometheus](#resetting-prometheus)
- [Generating Comparison Charts](#generating-comparison-charts)
- [Metrics](#metrics)
- [Monitoring](#monitoring)
- [Stopping the Platform](#stopping-the-platform)

## System Overview

The system implements a two-level cache architecture:

- **L1 cache**: a local in-memory cache in each Gateway instance;
- **L2 cache**: a shared Redis cache used by all Gateway instances;
- **Backend simulator**: a configurable service that simulates a slow or overloaded data source;
- **Load generator**: k6 scripts that reproduce different traffic patterns;
- **Monitoring layer**: Prometheus and Grafana;
- **Experiment tools**: Python scripts for running experiments, exporting metrics, and generating comparison charts.

The platform runs two Gateway instances behind an Nginx load balancer. This makes it possible to evaluate both local request coalescing and coordination between multiple Gateway instances.

## Architecture

The platform consists of the following services:

| Component | Description |
| --- | --- |
| Nginx | Distributes incoming requests between two Gateway instances |
| Gateway 1 and Gateway 2 | Implement L1/L2 caching and anti-stampede strategies |
| Redis | Shared L2 cache and distributed coordination storage |
| Backend | Simulates a slow or capacity-limited data source |
| Frontend | Provides runtime configuration of the cache and backend |
| Prometheus | Collects and stores system metrics |
| Grafana | Displays the provided monitoring dashboard |
| k6 | Generates controlled traffic patterns |

The request flow is:

```text
Client
  |
  v
Nginx load balancer
  |
  +----> Gateway 1 ----> Local L1 cache
  |
  +----> Gateway 2 ----> Local L1 cache
              |
              v
        Shared Redis L2 cache
              |
              v
       Backend simulator
```

## Supported Caching Strategies

The following operating modes are supported:

- **Baseline**  
  Standard cache-aside behavior without an additional anti-stampede mechanism.

- **SingleFlight**  
  Requests for the same key within one Gateway instance are coalesced into a single backend request.

- **Distributed Lock**  
  Gateway instances coordinate through Redis so that only one instance refreshes a missing value.

- **Probabilistic Early Refresh**  
  Cached values can be refreshed probabilistically before their expiration time.

- **Adaptive TTL**  
  Cache TTL values are dynamically scaled according to backend latency and concurrency signals.

Strategies are enabled and disabled through the frontend configuration page.

## Repository Structure

```text
distributed-caching/
├── backend/                  # Backend simulator
├── frontend/                 # Angular configuration interface
├── gateway/                  # Cache Gateway implementation
├── experiments/
│   ├── run_experiment.py     # Runs k6 and exports Prometheus metrics
│   ├── plot_results.py       # Generates comparison charts
│   └── results/              # Collected experiment results and comparison charts
├── monitoring/
│   ├── prometheus.yml        # Prometheus configuration
│   ├── dashboard-*.json      # Grafana dashboard
│   └── grafana/              # Grafana provisioning configuration
├── nginx/
│   └── nginx.conf            # Load balancer configuration
├── tests/                    # k6 load test scenarios
├── docker-compose.yml        # Experimental environment
└── reset-prometheus.bat      # Resets metrics between experiments
```

## Prerequisites

Install the following tools before running the platform:

- [Git](https://git-scm.com/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [Python 3.10 or newer](https://www.python.org/)
- [k6](https://grafana.com/docs/k6/latest/set-up/install-k6/)

The chart generation script also requires:

```powershell
pip install pandas matplotlib
```

Make sure that `docker`, `python`, and `k6` are available in the system `PATH`.

## Starting the Platform

Clone the repository:

```powershell
git clone <repository-url>
cd distributed-caching
```

Build and start all containers:

```powershell
docker compose up --build -d
```

Check the state of the containers:

```powershell
docker compose ps
```

Wait until the services are running before starting an experiment.

## Service URLs

| Service | URL |
| --- | --- |
| Frontend | [http://localhost:4200](http://localhost:4200) |
| Gateway/load balancer | [http://localhost:8080](http://localhost:8080) |
| Backend | [http://localhost:8081](http://localhost:8081) |
| Prometheus | [http://localhost:9090](http://localhost:9090) |
| Grafana | [http://localhost:3000](http://localhost:3000) |

The main data endpoint has the following form:

```text
GET http://localhost:8080/api/backend/{id}
```

For example:

```powershell
curl http://localhost:8080/api/backend/1
```

The Grafana dashboard and Prometheus datasource are provisioned automatically when the containers are started.

## Configuring the Platform

Open the frontend at:

```text
http://localhost:4200
```

The frontend can be used to configure:

- maximum number of L1 cache entries;
- maximum number of L2 cache entries;
- L1 TTL;
- L2 TTL;
- backend concurrency limit;
- backend base latency;
- additional latency per concurrent request;
- backend timeout;
- SingleFlight;
- distributed locking;
- probabilistic early refresh;
- early-refresh beta;
- adaptive TTL parameters.

Saving a new configuration clears the existing L1 and L2 cache entries. This ensures that the next experiment starts with the selected cache configuration.

Only the strategy being evaluated should be enabled. For example, the SingleFlight experiment should have SingleFlight enabled and the other anti-stampede strategies disabled.

## Experiment Configurations

The platform configuration is set through the frontend before each experiment.

The cache and backend parameters remain the same when comparing strategies within one load scenario. Only the evaluated anti-stampede strategy is enabled.

### Moderate and High-Skew Zipf Configuration

The following configuration is used for both Zipf scenarios:

- `tests/moderate-skew-zipf.js`, with a Zipf skew of `0.8`;
- `tests/high-skew-zipf.js`, with a Zipf skew of `1.2`.

| Parameter | Value |
| --- | ---: |
| L1 maximum entries | 30 |
| L2 maximum entries | 60 |
| L1 TTL | 5 seconds |
| L2 TTL | 15 seconds |
| Backend semaphore size | 5 |
| Delay per concurrent backend request | 10 ms |
| Backend base latency | 200 ms |
| Backend timeout | 1500 ms |

### Synchronized-Expiry Configuration

The following configuration is used with `tests/synchronized-expiry.js`:

| Parameter | Value |
| --- | ---: |
| L1 maximum entries | 10 |
| L2 maximum entries | 10 |
| L1 TTL | 1 second |
| L2 TTL | 10 seconds |
| Backend semaphore size | 5 |
| Delay per concurrent backend request | 10 ms |
| Backend base latency | 200 ms |
| Backend timeout | 1500 ms |

### Burst-Load Configuration

The following configuration is used with `tests/burst-load.js`:

| Parameter | Value |
| --- | ---: |
| L1 maximum entries | 50 |
| L2 maximum entries | 120 |
| L1 TTL | 5 seconds |
| L2 TTL | 15 seconds |
| Backend semaphore size | 10 |
| Delay per concurrent backend request | 10 ms |
| Backend base latency | 200 ms |
| Early-refresh beta | 1.0 |
| Backend timeout | 1500 ms |

For the adaptive TTL variant of the burst-load experiment, the following additional parameters are used:

| Adaptive TTL parameter | Value |
| --- | ---: |
| Minimum factor | 0.8 |
| Maximum factor | 2.5 |
| Backend P99 latency threshold | 280 ms |
| Backend concurrency threshold | 6 |
| Adjustment interval | 2000 ms |

### Backend-Unavailability Configuration

The following configuration is used with `tests/backend-unavailability.js`:

| Parameter | Value |
| --- | ---: |
| L1 maximum entries | 30 |
| L2 maximum entries | 60 |
| L1 TTL | 5 seconds |
| L2 TTL | 15 seconds |
| Backend semaphore size | 5 |
| Delay per concurrent backend request | 10 ms |
| Backend base latency | 200 ms |
| Early-refresh beta | 0.1 |
| Backend timeout | 1500 ms |

For the adaptive TTL variant of the backend-unavailability experiment, the following additional parameters are used:

| Adaptive TTL parameter | Value |
| --- | ---: |
| Minimum factor | 0.8 |
| Maximum factor | 2.5 |
| Backend P99 latency threshold | 280 ms |
| Backend concurrency threshold | 6 |
| Adjustment interval | 2000 ms |

### L2-Only Zipf Configuration

The following configuration is used with `tests/moderate-skew-zipf-l2only.js`:

| Parameter | Value |
| --- | ---: |
| L1 maximum entries | 0 (disabled) |
| L2 maximum entries | 60 |
| L1 TTL | 5 seconds (unused) |
| L2 TTL | 15 seconds |
| Backend semaphore size | 5 |
| Delay per concurrent backend request | 10 ms |
| Backend base latency | 200 ms |
| Early-refresh beta | 1.0 |
| Backend timeout | 1500 ms |

L1 is disabled through the frontend before the experiment. The results are compared with the moderate-skew Zipf scenario using both L1 and L2 caches.

### Strategy Selection

Only the strategy being evaluated should be enabled. All other strategies should remain disabled.

| Experiment name | SingleFlight | Distributed lock | Early refresh | Adaptive TTL |
| --- | :---: | :---: | :---: | :---: |
| `baseline` | Off | Off | Off | Off |
| `singleFlight` | On | Off | Off | Off |
| `distributedLock` | Off | On | Off | Off |
| `earlyRefresh` | Off | Off | On | Off |
| `adaptiveTTL` | Off | Off | Off | On |

The `adaptiveTTL` variant is used for the burst-load and backend-unavailability scenarios. The cache and backend parameters for each scenario are retained, while the corresponding adaptive TTL settings shown above are enabled.

## Load Test Scenarios

The repository contains the following k6 scenarios:

| Test file | Scenario |
| --- | --- |
| `tests/moderate-skew-zipf.js` | Moderate Zipf distribution |
| `tests/high-skew-zipf.js` | High-skew Zipf distribution with stronger hot-key behavior |
| `tests/synchronized-expiry.js` | Simultaneous expiration of several popular keys |
| `tests/burst-load.js` | Sudden traffic increase followed by a recovery period |
| `tests/backend-unavailability.js` | Temporary backend unavailability followed by recovery |
| `tests/moderate-skew-zipf-l2only.js` | Moderate Zipf distribution with L1 disabled |

### Moderate Zipf load

`tests/moderate-skew-zipf.js` uses:

- 100 keys;
- Zipf skew: `0.8`;
- warm-up from 5 to 78 requests per second;
- 78 requests per second during the main phase;
- total test duration: 240 seconds.

### High-skew Zipf load

`tests/high-skew-zipf.js` uses:

- 100 keys;
- Zipf skew: `1.2`;
- warm-up from 5 to 78 requests per second;
- 78 requests per second during the main phase;
- total test duration: 240 seconds.

### Synchronized-expiry load

`tests/synchronized-expiry.js` uses:

- 5 popular keys;
- Zipf skew: `1.2`;
- 80 requests per second;
- test duration: 60 seconds;
- an initial batch request that loads all selected keys into the cache with approximately synchronized expiration times.

### Burst load

`tests/burst-load.js` uses:

- 100 keys;
- Zipf skew: `0.8`;
- baseline rate: 20 requests per second;
- burst rate: 180 requests per second;
- 30-second baseline phase;
- 1-second ramp-up;
- 45-second burst phase;
- 1-second ramp-down;
- 45-second recovery phase.

### Backend unavailability

`tests/backend-unavailability.js` uses:

- 100 keys;
- Zipf skew: `0.8`;
- 15-second warm-up from 5 to 77 requests per second;
- 77 requests per second during the 90-second main phase;
- total test duration: 105 seconds;
- a 13-second backend fault activated at 45 seconds.

### L2-only Zipf load

`tests/moderate-skew-zipf-l2only.js` uses:

- 100 keys;
- Zipf skew: `0.8`;
- warm-up from 5 to 78 requests per second;
- 78 requests per second during the main phase;
- total test duration: 240 seconds;
- L2-only caching, compared with the L1 + L2 configuration under the same load profile.

## Running an Experiment

Before running an experiment:

1. Start all containers.
2. Open the frontend.
3. Apply the required cache and backend configuration.
4. Enable the strategy being evaluated.
5. Confirm that Prometheus is available at `http://localhost:9090`.

Move to the experiment directory:

```powershell
cd experiments
```

Run an experiment with:

```powershell
python run_experiment.py --name baseline --test tests/moderate-skew-zipf.js
```

Although the command is executed from the `experiments` directory, test paths are resolved relative to the repository root.

The `--name` argument identifies the tested strategy. Example names are:

```text
baseline
singleFlight
distributedLock
earlyRefresh
adaptiveTTL
```

For example, after enabling SingleFlight in the frontend:

```powershell
python run_experiment.py --name singleFlight --test tests/moderate-skew-zipf.js
```

After enabling distributed locking:

```powershell
python run_experiment.py --name distributedLock --test tests/moderate-skew-zipf.js
```

To run another traffic scenario, change the `--test` argument:

```powershell
python run_experiment.py --name baseline --test tests/high-skew-zipf.js
python run_experiment.py --name baseline --test tests/synchronized-expiry.js
python run_experiment.py --name baseline --test tests/burst-load.js
python run_experiment.py --name baseline --test tests/backend-unavailability.js
python run_experiment.py --name baseline --test tests/moderate-skew-zipf-l2only.js
```

## Experiment Results

The repository already contains the collected results for all evaluated load scenarios and caching strategies. The results can be found in the `experiments/results` directory, organized by load scenario and experiment run.

The included results cover:

- moderate-skew Zipf load;
- high-skew Zipf load;
- synchronized-expiry load;
- burst load;
- backend unavailability;
- moderate-skew Zipf load with L2-only caching;
- baseline behavior;
- SingleFlight;
- distributed locking;
- probabilistic early refresh;
- adaptive TTL evaluation for the burst-load and backend-unavailability scenarios.

Experiment data is written to:

```text
experiments/results/<test-name>/<timestamp>_<experiment-name>/
```

For example:

```text
experiments/results/zipf1/
└── 2026-08-14_10-19-39-459154_baseline/
    ├── k6-summary.json
    ├── metadata.json
    ├── metrics.csv
    ├── prometheus-queries.json
    └── system-config.json
```

Each experiment directory contains:

| File | Description |
| --- | --- |
| `k6-summary.json` | k6 execution summary |
| `metadata.json` | Experiment name, duration, test file, timestamps, and environment information |
| `metrics.csv` | Time-series metrics exported from Prometheus |
| `prometheus-queries.json` | PromQL expressions used during metric export |
| `system-config.json` | Experiment configuration and environment information |

A separate timestamped folder is created for every execution, so previous experiment results are not overwritten.

## Resetting Prometheus

Prometheus and application metric counters must be reset after every completed experiment and before the next experiment is started.

From the `experiments` directory, run:

```powershell
..\reset-prometheus.bat
```

From the repository root, run:

```powershell
.\reset-prometheus.bat
```

The script:

- restarts the backend service;
- restarts both Gateway instances;
- removes and recreates the Prometheus container.

This resets accumulated Prometheus counters so that values from the previous experiment do not affect the next experiment.

After the reset is complete:

1. wait for Prometheus and the application services to become available;
2. open the frontend;
3. apply the configuration for the next strategy;
4. start the next experiment.

The recommended sequence is:

```text
Configure strategy
        |
        v
Run experiment
        |
        v
Save results automatically
        |
        v
Reset Prometheus
        |
        v
Configure the next strategy
```

## Generating Comparison Charts

After all strategies for one traffic scenario have been executed, generate a comparison image using `plot_results.py`.

From the `experiments` directory:

```powershell
python plot_results.py results/zipf1
```

Other examples:

```powershell
python plot_results.py results/high-skew-zipf
python plot_results.py results/synchronized-expiry
python plot_results.py results/burst-load
python plot_results.py results/backend-unavailability
python plot_results.py results/moderate-skew-zipf-l2only
```

By default, the generated image is saved as:

```text
experiments/results/<test-name>/comparison.jpg
```

The script loads all `metrics.csv` files found under the selected scenario folder and draws the results of different strategies on comparative time-series charts.

## Metrics

### Cache Hit Ratio

The total Cache Hit Ratio is calculated as:

```text
CHR = (L1 hits + L2 hits) / total user requests
```

The individual cache-level ratios are:

```text
L1 CHR = L1 hits / total user requests
L2 CHR = L2 hits / total user requests
```

### Failure Amplification Factor

The aggregate Failure Amplification Factor is defined as:

```text
FAF = number of backend requests / number of user requests
```

Lower values indicate that the cache absorbs a larger portion of user traffic.

### Failure Amplification Factor by Key

For a selected key `k`:

```text
FAF(k) =
    number of backend calls caused by requests for key k
    ----------------------------------------------------
    number of logical refreshes required for key k
```

A value close to `1` indicates that approximately one backend request was performed for one required refresh. Higher values indicate redundant backend calls and a possible cache stampede.

### Latency

The platform records:

- average user request duration;
- P95 user request duration;
- P99 user request duration;
- average backend request duration.

### Additional Metrics

The monitoring and experiment tools also collect:

- total user requests;
- total backend requests;
- backend throughput;
- active backend requests;
- cache misses;
- L1 hits;
- L2 hits;
- failed-request ratio;
- current SingleFlight waiting requests;
- successful distributed lock attempts;
- failed distributed lock attempts;
- adaptive TTL state;
- adaptive TTL scaling factor;
- effective L1 and L2 TTL;
- backend P99 signal used by the adaptive TTL controller.

## Monitoring

Prometheus is available at:

```text
http://localhost:9090
```

Grafana is available at:

```text
http://localhost:3000
```

The Prometheus datasource and distributed caching dashboard are provisioned automatically through the files in:

```text
monitoring/grafana/provisioning/
```

The dashboard definition is stored in:

```text
monitoring/dashboard-1785416027985.json
```

Grafana can be used to observe system behavior while an experiment is running. The Python experiment runner exports the same key metrics to CSV for later comparison and chart generation.

## Stopping the Platform

Stop all containers:

```powershell
docker compose down
```

To stop the containers and remove their volumes:

```powershell
docker compose down -v
```

Removing volumes also removes stored Redis and Prometheus data.
