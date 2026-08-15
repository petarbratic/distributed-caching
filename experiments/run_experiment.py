#!/usr/bin/env python3

import argparse
import csv
import json
import platform
import re
import subprocess
import sys
import time
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


PROJECT_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_RESULTS_DIR = PROJECT_ROOT / "experiments" / "results"
DEFAULT_PROMETHEUS_URL = "http://localhost:9090"


# PromQL expressions taken from the Grafana dashboard.
#
# Grafana variables:
#   $__rate_interval -> replaced with the --rate-window value
#   $__range         -> replaced with the experiment duration
#
PROMETHEUS_QUERIES = {
    "all_requests": """
        sum(
          gateway_http_requests_total{job="gateway"}
        )
    """,

    "l1_hits": """
        sum(
          gateway_l1_hits_total{job="gateway"}
        )
    """,

    "l2_hits": """
        sum(
          gateway_l2_hits_total{job="gateway"}
        )
    """,

    "backend_calls": """
        sum(
          backend_requests_total{job="backend"}
        )
    """,

    "cache_misses": """
        sum(
          gateway_cache_miss_total{job="gateway"}
        )
    """,

    "backend_throughput": """
        sum(
          rate(
            backend_requests_total{
              job="backend"
            }[$__rate_interval]
          )
        )
    """,

    "active_backend_requests": """
        sum(
          backend_active_requests{
            job="backend"
          }
        )
    """,

    "failed_request_ratio": """
        sum(
          rate(
            gateway_failed_requests_total{
              job="gateway"
            }[$__rate_interval]
          )
        )
        /
        clamp_min(
          sum(
            rate(
              gateway_http_requests_total{
                job="gateway"
              }[$__rate_interval]
            )
          ),
          1e-9
        )
    """,

    "singleflight_waiting_requests": """
        sum(
          gateway_singleflight_waiting_requests{
            job="gateway"
          }
        )
    """,

    "distributed_lock_successful": """
        sum(
          gateway_distributed_lock_attempts_total{
            job="gateway",
            result="success"
          }
        )
    """,

    "distributed_lock_failed": """
        sum(
          gateway_distributed_lock_attempts_total{
            job="gateway",
            result="failure"
          }
        )
    """,

    "faf_by_key": """
        sum by (key) (
          gateway_backend_calls_total{
            job="gateway",
            key=~"/api/backend/(1|2|3|4|5|6)"
          }
        )
        /
        clamp_min(
          sum by (key) (
            gateway_logical_refreshes_total{
              job="gateway",
              key=~"/api/backend/(1|2|3|4|5|6)"
            }
          ),
          1
        )
    """,

    "average_request_duration": """
        sum(
          rate(
            gateway_request_duration_seconds_sum{
              job="gateway"
            }[$__rate_interval]
          )
        )
        /
        clamp_min(
          sum(
            rate(
              gateway_request_duration_seconds_count{
                job="gateway"
              }[$__rate_interval]
            )
          ),
          1e-9
        )
    """,

    "average_backend_duration": """
        sum(
          rate(
            gateway_backend_duration_seconds_sum{
              job="gateway"
            }[$__rate_interval]
          )
        )
        /
        clamp_min(
          sum(
            rate(
              gateway_backend_duration_seconds_count{
                job="gateway"
              }[$__rate_interval]
            )
          ),
          1e-9
        )
    """,

    "failure_amplification_factor": """
        sum(
          increase(
            backend_requests_total{
              job="backend"
            }[$__range]
          )
        )
        /
        clamp_min(
          sum(
            increase(
              gateway_http_requests_total{
                job="gateway"
              }[$__range]
            )
          ),
          1
        )
    """,

    "cache_hit_ratio_total": """
        (
          sum(
            rate(
              gateway_l1_hits_total{
                job="gateway"
              }[$__rate_interval]
            )
          )
          +
          sum(
            rate(
              gateway_l2_hits_total{
                job="gateway"
              }[$__rate_interval]
            )
          )
        )
        /
        clamp_min(
          sum(
            rate(
              gateway_http_requests_total{
                job="gateway"
              }[$__rate_interval]
            )
          ),
          1e-9
        )
    """,

    "cache_hit_ratio_l1": """
        sum(
          rate(
            gateway_l1_hits_total{
              job="gateway"
            }[$__rate_interval]
          )
        )
        /
        clamp_min(
          sum(
            rate(
              gateway_http_requests_total{
                job="gateway"
              }[$__rate_interval]
            )
          ),
          1e-9
        )
    """,

    "cache_hit_ratio_l2": """
        sum(
          rate(
            gateway_l2_hits_total{
              job="gateway"
            }[$__rate_interval]
          )
        )
        /
        clamp_min(
          sum(
            rate(
              gateway_http_requests_total{
                job="gateway"
              }[$__rate_interval]
            )
          ),
          1e-9
        )
    """,

    "request_duration_p95": """
        histogram_quantile(
          0.95,
          sum by (le) (
            rate(
              gateway_request_duration_seconds_bucket{
                job="gateway"
              }[$__rate_interval]
            )
          )
        )
    """,

    "request_duration_p99": """
        histogram_quantile(
          0.99,
          sum by (le) (
            rate(
              gateway_request_duration_seconds_bucket{
                job="gateway"
              }[$__rate_interval]
            )
          )
        )
    """,
     "adaptive_ttl_factor": """
        avg(
          gateway_adaptive_ttl_factor{
            job="gateway"
          }
        )
    """,

    "adaptive_ttl_effective_seconds": """
        avg by (level) (
          gateway_adaptive_ttl_effective_seconds{
            job="gateway"
          }
        )
    """,

    "adaptive_ttl_state": """
        avg(
          gateway_adaptive_ttl_state{
            job="gateway"
          }
        )
    """,

    "adaptive_ttl_backend_p99_milliseconds": """
        avg(
          gateway_adaptive_ttl_backend_p99_milliseconds{
            job="gateway"
          }
        )
    """,
}


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def to_iso(value: datetime) -> str:
    return value.isoformat().replace("+00:00", "Z")


def sanitize_name(value: str) -> str:
    sanitized = re.sub(r"[^a-zA-Z0-9_-]+", "-", value.strip())
    sanitized = sanitized.strip("-_")

    if not sanitized:
        raise ValueError("The name must not be empty.")

    return sanitized


def normalize_promql(query: str) -> str:
    return " ".join(query.split())


def format_prometheus_duration(seconds: float) -> str:
    # A Prometheus duration must be at least one second.
    return f"{max(1, int(round(seconds)))}s"


def prepare_promql(
    query: str,
    rate_window: str,
    experiment_duration_seconds: float,
) -> str:
    return normalize_promql(
        query
        .replace("$__rate_interval", rate_window)
        .replace(
            "$__range",
            format_prometheus_duration(experiment_duration_seconds),
        )
    )


def write_json(path: Path, data: Any) -> None:
    with path.open("w", encoding="utf-8") as file:
        json.dump(data, file, indent=2, ensure_ascii=False)
        file.write("\n")


def load_json_config(path: Path | None) -> dict[str, Any]:
    if path is None:
        return {}

    if not path.is_file():
        raise FileNotFoundError(
            f"Configuration file does not exist: {path}"
        )

    with path.open("r", encoding="utf-8") as file:
        data = json.load(file)

    if not isinstance(data, dict):
        raise ValueError(
            "The configuration file must contain a JSON object."
        )

    return data


def run_optional_command(command: list[str]) -> str | None:
    try:
        result = subprocess.run(
            command,
            cwd=PROJECT_ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
    except (FileNotFoundError, OSError):
        return None

    if result.returncode != 0:
        return None

    return result.stdout.strip() or None


def collect_environment_info() -> dict[str, Any]:
    return {
        "operating_system": platform.platform(),
        "python_version": sys.version,
        "git_commit": run_optional_command(
            ["git", "rev-parse", "HEAD"]
        ),
        "git_branch": run_optional_command(
            ["git", "branch", "--show-current"]
        ),
        "k6_version": run_optional_command(
            ["k6", "version"]
        ),
        "docker_version": run_optional_command(
            ["docker", "--version"]
        ),
    }


def check_prometheus(prometheus_url: str) -> None:
    endpoint = f"{prometheus_url.rstrip('/')}/-/ready"

    try:
        with urllib.request.urlopen(endpoint, timeout=5) as response:
            if response.status != 200:
                raise RuntimeError(
                    f"Prometheus returned HTTP {response.status}."
                )
    except Exception as error:
        raise RuntimeError(
            f"Prometheus is not available at "
            f"{prometheus_url}: {error}"
        ) from error


def create_experiment_directory(
    results_root: Path,
    test_path: Path,
    experiment_name: str,
    started_at: datetime,
) -> tuple[str, str, Path]:
    test_name = sanitize_name(test_path.stem)

    timestamp = started_at.strftime("%Y-%m-%d_%H-%M-%S-%f")
    experiment_id = f"{timestamp}_{experiment_name}"

    experiment_directory = (
        results_root
        / test_name
        / experiment_id
    )

    experiment_directory.mkdir(
        parents=True,
        exist_ok=False,
    )

    return test_name, experiment_id, experiment_directory


def run_k6(
    test_path: Path,
    summary_path: Path,
    additional_arguments: list[str],
) -> int:
    command = [
        "k6",
        "run",
        "--summary-export",
        str(summary_path),
        *additional_arguments,
        str(test_path),
    ]

    print()
    print("Running:")
    print(subprocess.list2cmdline(command))
    print()

    try:
        completed_process = subprocess.run(
            command,
            cwd=PROJECT_ROOT,
            check=False,
        )
    except FileNotFoundError as error:
        raise RuntimeError(
            "The 'k6' command was not found. Check that k6 is "
            "installed and available in PATH."
        ) from error

    return completed_process.returncode


def query_prometheus_range(
    prometheus_url: str,
    query: str,
    start_timestamp: float,
    end_timestamp: float,
    step_seconds: int,
) -> list[dict[str, Any]]:
    parameters = urllib.parse.urlencode(
        {
            "query": query,
            "start": f"{start_timestamp:.6f}",
            "end": f"{end_timestamp:.6f}",
            "step": str(step_seconds),
        }
    )

    endpoint = (
        f"{prometheus_url.rstrip('/')}/api/v1/query_range?"
        f"{parameters}"
    )

    request = urllib.request.Request(
        endpoint,
        headers={"Accept": "application/json"},
    )

    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            payload = json.loads(
                response.read().decode("utf-8")
            )
    except Exception as error:
        raise RuntimeError(
            f"Prometheus query failed: {error}\n"
            f"PromQL: {query}"
        ) from error

    if payload.get("status") != "success":
        raise RuntimeError(
            f"Prometheus returned an error: {payload}\n"
            f"PromQL: {query}"
        )

    data = payload.get("data", {})

    if data.get("resultType") != "matrix":
        raise RuntimeError(
            "Expected a Prometheus matrix result, "
            f"but received: {data.get('resultType')}"
        )

    return data.get("result", [])


def labels_to_series(labels: dict[str, str]) -> str:
    relevant_labels = {
        key: value
        for key, value in labels.items()
        if key != "__name__"
    }

    if not relevant_labels:
        return "all"

    return ",".join(
        f"{key}={value}"
        for key, value in sorted(relevant_labels.items())
    )


def export_prometheus_metrics(
    output_path: Path,
    prometheus_url: str,
    experiment_id: str,
    experiment_name: str,
    test_name: str,
    started_at: datetime,
    ended_at: datetime,
    step_seconds: int,
    rate_window: str,
) -> tuple[dict[str, int], dict[str, str]]:
    start_timestamp = started_at.timestamp()
    end_timestamp = ended_at.timestamp()
    duration_seconds = (
        ended_at - started_at
    ).total_seconds()

    exported_rows: dict[str, int] = {}
    executed_queries: dict[str, str] = {}

    fieldnames = [
        "experiment_id",
        "experiment_name",
        "test_name",
        "timestamp",
        "relative_time_seconds",
        "metric",
        "series",
        "value",
    ]

    with output_path.open(
        "w",
        encoding="utf-8",
        newline="",
    ) as file:
        writer = csv.DictWriter(
            file,
            fieldnames=fieldnames,
        )
        writer.writeheader()

        for metric_name, query_template in PROMETHEUS_QUERIES.items():
            query = prepare_promql(
                query=query_template,
                rate_window=rate_window,
                experiment_duration_seconds=duration_seconds,
            )

            executed_queries[metric_name] = query

            print(f"Fetching: {metric_name}")

            results = query_prometheus_range(
                prometheus_url=prometheus_url,
                query=query,
                start_timestamp=start_timestamp,
                end_timestamp=end_timestamp,
                step_seconds=step_seconds,
            )

            row_count = 0

            for result in results:
                series = labels_to_series(
                    result.get("metric", {})
                )

                for timestamp, value in result.get("values", []):
                    timestamp = float(timestamp)

                    writer.writerow(
                        {
                            "experiment_id": experiment_id,
                            "experiment_name": experiment_name,
                            "test_name": test_name,
                            "timestamp": f"{timestamp:.6f}",
                            "relative_time_seconds": (
                                f"{timestamp - start_timestamp:.6f}"
                            ),
                            "metric": metric_name,
                            "series": series,
                            "value": value,
                        }
                    )

                    row_count += 1

            exported_rows[metric_name] = row_count

    return exported_rows, executed_queries


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Runs a k6 test and exports results from Prometheus."
        )
    )

    parser.add_argument(
        "--name",
        required=True,
        help="Experiment name, for example baseline.",
    )

    parser.add_argument(
        "--test",
        required=True,
        type=Path,
        help="Path to the k6 JavaScript test.",
    )

    parser.add_argument(
        "--config",
        type=Path,
        help=(
            "Optional JSON file containing the system configuration "
            "used during the experiment."
        ),
    )

    parser.add_argument(
        "--prometheus-url",
        default=DEFAULT_PROMETHEUS_URL,
        help=(
            "Prometheus URL. Default: "
            f"{DEFAULT_PROMETHEUS_URL}"
        ),
    )

    parser.add_argument(
        "--results-dir",
        type=Path,
        default=DEFAULT_RESULTS_DIR,
        help="Root directory for experiment results.",
    )

    parser.add_argument(
        "--step",
        type=int,
        default=1,
        help=(
            "Interval between Prometheus samples in seconds. "
            "Default: 1."
        ),
    )

    parser.add_argument(
        "--rate-window",
        default="10s",
        help=(
            "Prometheus window used in rate() expressions. "
            "Replaces Grafana's $__rate_interval. Default: 10s."
        ),
    )

    parser.add_argument(
        "--scrape-wait",
        type=float,
        default=2.0,
        help=(
            "Time to wait after the k6 test so Prometheus can "
            "complete its final scrape."
        ),
    )

    parser.add_argument(
        "k6_args",
        nargs=argparse.REMAINDER,
        help="Additional k6 arguments specified after --.",
    )

    return parser.parse_args()


def main() -> int:
    args = parse_arguments()

    metadata_path: Path | None = None
    metadata: dict[str, Any] | None = None

    try:
        experiment_name = sanitize_name(args.name)

        test_path = args.test
        if not test_path.is_absolute():
            test_path = PROJECT_ROOT / test_path
        test_path = test_path.resolve()

        if not test_path.is_file():
            raise FileNotFoundError(
                f"k6 test does not exist: {test_path}"
            )

        if test_path.suffix.lower() != ".js":
            raise ValueError(
                "The k6 test must be a JavaScript file with a .js extension."
            )

        config_path = args.config
        if config_path is not None:
            if not config_path.is_absolute():
                config_path = PROJECT_ROOT / config_path
            config_path = config_path.resolve()

        results_root = args.results_dir
        if not results_root.is_absolute():
            results_root = PROJECT_ROOT / results_root
        results_root = results_root.resolve()

        if args.step <= 0:
            raise ValueError("--step must be greater than zero.")

        if args.scrape_wait < 0:
            raise ValueError(
                "--scrape-wait cannot be negative."
            )

        system_config = load_json_config(config_path)

        check_prometheus(args.prometheus_url)

        k6_arguments = list(args.k6_args)
        if k6_arguments and k6_arguments[0] == "--":
            k6_arguments = k6_arguments[1:]

        # Record the start immediately before launching the k6 process.
        started_at = utc_now()

        (
            test_name,
            experiment_id,
            experiment_directory,
        ) = create_experiment_directory(
            results_root=results_root,
            test_path=test_path,
            experiment_name=experiment_name,
            started_at=started_at,
        )

        metadata_path = experiment_directory / "metadata.json"
        system_config_path = (
            experiment_directory / "system-config.json"
        )
        metrics_path = experiment_directory / "metrics.csv"
        queries_path = (
            experiment_directory / "prometheus-queries.json"
        )
        k6_summary_path = (
            experiment_directory / "k6-summary.json"
        )

        try:
            relative_test_path = str(
                test_path.relative_to(PROJECT_ROOT)
            )
        except ValueError:
            relative_test_path = str(test_path)

        metadata = {
            "experiment_id": experiment_id,
            "name": experiment_name,
            "test_name": test_name,
            "test_file": relative_test_path,
            "status": "running",
            "started_at": to_iso(started_at),
            "ended_at": None,
            "duration_seconds": None,
            "k6_arguments": k6_arguments,
            "k6_exit_code": None,
            "prometheus": {
                "url": args.prometheus_url,
                "step_seconds": args.step,
                "rate_window": args.rate_window,
                "scrape_wait_seconds": args.scrape_wait,
            },
        }

        write_json(metadata_path, metadata)

        write_json(
            system_config_path,
            {
                "experiment_config": system_config,
                "environment": collect_environment_info(),
            },
        )

        print(f"Experiment: {experiment_id}")
        print(f"Test: {test_name}")
        print(f"Started at: {to_iso(started_at)}")
        print(f"Results: {experiment_directory}")

        k6_exit_code = run_k6(
            test_path=test_path,
            summary_path=k6_summary_path,
            additional_arguments=k6_arguments,
        )

        # The experiment ends when the k6 process exits.
        ended_at = utc_now()

        print()
        print(f"Test ended at: {to_iso(ended_at)}")
        print(
            f"Waiting {args.scrape_wait:g} s for the final "
            "Prometheus scrape..."
        )

        time.sleep(args.scrape_wait)

        exported_rows, executed_queries = (
            export_prometheus_metrics(
                output_path=metrics_path,
                prometheus_url=args.prometheus_url,
                experiment_id=experiment_id,
                experiment_name=experiment_name,
                test_name=test_name,
                started_at=started_at,
                ended_at=ended_at,
                step_seconds=args.step,
                rate_window=args.rate_window,
            )
        )

        write_json(queries_path, executed_queries)

        duration_seconds = (
            ended_at - started_at
        ).total_seconds()

        metadata.update(
            {
                "status": (
                    "completed"
                    if k6_exit_code == 0
                    else "k6_failed"
                ),
                "ended_at": to_iso(ended_at),
                "duration_seconds": duration_seconds,
                "k6_exit_code": k6_exit_code,
                "exported_rows": exported_rows,
            }
        )

        write_json(metadata_path, metadata)

        print()
        print("Experiment completed.")
        print(f"Status: {metadata['status']}")
        print(f"Duration: {duration_seconds:.3f} s")
        print(f"Results: {experiment_directory}")

        return k6_exit_code

    except KeyboardInterrupt:
        if metadata is not None and metadata_path is not None:
            metadata["status"] = "interrupted"
            metadata["ended_at"] = to_iso(utc_now())
            write_json(metadata_path, metadata)

        print(
            "\nExperiment interrupted by the user.",
            file=sys.stderr,
        )
        return 130

    except Exception as error:
        if metadata is not None and metadata_path is not None:
            metadata["status"] = "failed"
            metadata["ended_at"] = to_iso(utc_now())
            metadata["error"] = str(error)
            write_json(metadata_path, metadata)

        print(f"Error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
