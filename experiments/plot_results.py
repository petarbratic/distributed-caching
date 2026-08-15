#!/usr/bin/env python3

import argparse
import math
import sys
from pathlib import Path

import matplotlib

# Generate the image without opening a graphical window.
matplotlib.use("Agg")

import matplotlib.pyplot as plt
import pandas as pd
from matplotlib.ticker import ScalarFormatter


REQUIRED_COLUMNS = {
    "experiment_id",
    "experiment_name",
    "relative_time_seconds",
    "metric",
    "series",
    "value",
}

METRIC_TITLES = {
    "all_requests": "Укупан број захтева",
    "l1_hits": "L1 погоци",
    "l2_hits": "L2 погоци",
    "backend_calls": "Позиви backend сервису",
    "cache_misses": "Cache промашаји",
    "backend_throughput": "Backend проток",
    "active_backend_requests": "Активни backend захтеви",
    "failed_request_ratio": "Удео неуспешних захтева",
    "singleflight_waiting_requests": (
        "Захтеви који чекају SingleFlight резултат"
    ),
    "distributed_lock_successful": (
        "Успешни покушаји Distributed Lock-а"
    ),
    "distributed_lock_failed": (
        "Неуспешни покушаји Distributed Lock-а"
    ),
    "distributed_lock_attempts": "Покушаји Distributed Lock-а",
    "faf_by_key": "FAF по кључу",
    "average_request_duration": (
        "Просечно трајање захтева"
    ),
    "average_backend_duration": (
        "Просечно трајање backend позива"
    ),
    "failure_amplification_factor": (
        "FAF"
    ),
    "cache_hit_ratio_total": "Укупни CHR",
    "cache_hit_ratio_l1": "L1 CHR",
    "cache_hit_ratio_l2": "L2 CHR",
    "request_duration_p95": "P95 трајање захтева",
    "request_duration_p99": "P99 трајање захтева",
    
    "adaptive_ttl_factor": "Adaptive TTL фактор k",
    "adaptive_ttl_effective_seconds": "Ефективни TTL",
    "adaptive_ttl_state": "Стање Adaptive TTL контролера",
    "adaptive_ttl_backend_p99_milliseconds": (
        "Backend P99 сигнал Adaptive TTL контролера"
    ),
}

METRIC_UNITS = {
    "all_requests": "Број захтева",
    "l1_hits": "Број погодака",
    "l2_hits": "Број погодака",
    "backend_calls": "Број позива",
    "cache_misses": "Број промашаја",
    "backend_throughput": "Захтева/s",
    "active_backend_requests": "Број захтева",
    "failed_request_ratio": "Однос",
    "singleflight_waiting_requests": "Број захтева",
    "distributed_lock_successful": "Број покушаја",
    "distributed_lock_failed": "Број покушаја",
    "distributed_lock_attempts": "Број покушаја",
    "faf_by_key": "FAF",
    "average_request_duration": "Секунде",
    "average_backend_duration": "Секунде",
    "failure_amplification_factor": "FAF",
    "cache_hit_ratio_total": "Однос",
    "cache_hit_ratio_l1": "Однос",
    "cache_hit_ratio_l2": "Однос",
    "request_duration_p95": "Секунде",
    "request_duration_p99": "Секунде",
    "adaptive_ttl_factor": "Фактор",
    "adaptive_ttl_effective_seconds": "Секунде",
    "adaptive_ttl_state": "Стање",
    "adaptive_ttl_backend_p99_milliseconds": "Милисекунде",
}

# Order in which metrics appear in the image.
METRIC_ORDER = [
    "all_requests",
    "l1_hits",
    "l2_hits",
    "backend_calls",
    "cache_misses",
    "backend_throughput",
    "active_backend_requests",
    "failed_request_ratio",
    "singleflight_waiting_requests",
    "distributed_lock_attempts",
    "faf_by_key",
    "average_request_duration",
    "average_backend_duration",
    "failure_amplification_factor",
    "cache_hit_ratio_total",
    "cache_hit_ratio_l1",
    "cache_hit_ratio_l2",
    "request_duration_p95",
    "request_duration_p99",
    "adaptive_ttl_backend_p99_milliseconds",
    "adaptive_ttl_state",
    "adaptive_ttl_factor",
    "adaptive_ttl_effective_seconds",
]


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Finds all metrics.csv files for a selected test "
            "and creates one comparative JPG image."
        )
    )

    parser.add_argument(
        "results_folder",
        type=Path,
        help=(
            "Test folder inside the results directory, "
            "for example results/zipf1."
        ),
    )

    parser.add_argument(
        "--output",
        default="comparison.jpg",
        help=(
            "Output JPG filename. Default: comparison.jpg."
        ),
    )

    parser.add_argument(
        "--columns",
        type=int,
        default=2,
        help=(
            "Number of chart columns in the image. Default: 2."
        ),
    )

    parser.add_argument(
        "--dpi",
        type=int,
        default=200,
        help=(
            "Output image resolution. Default: 200 DPI."
        ),
    )

    parser.add_argument(
        "--title",
        help=(
            "Optional title for the entire image. If omitted, "
            "the test folder name is used."
        ),
    )

    return parser.parse_args()


def resolve_results_folder(path: Path) -> Path:
    resolved = path.expanduser().resolve()

    if not resolved.exists():
        raise FileNotFoundError(
            f"Folder does not exist: {resolved}"
        )

    if not resolved.is_dir():
        raise NotADirectoryError(
            f"The provided path is not a folder: {resolved}"
        )

    return resolved


def find_metrics_files(results_folder: Path) -> list[Path]:
    return sorted(
        path
        for path in results_folder.rglob("metrics.csv")
        if path.is_file()
    )


def load_metrics_file(csv_path: Path) -> pd.DataFrame:
    try:
        frame = pd.read_csv(csv_path)
    except Exception as error:
        raise ValueError(
            f"Could not read CSV: {error}"
        ) from error

    missing_columns = REQUIRED_COLUMNS - set(frame.columns)

    if missing_columns:
        missing = ", ".join(sorted(missing_columns))
        raise ValueError(
            f"Missing columns: {missing}"
        )

    if frame.empty:
        raise ValueError("CSV is empty.")

    frame = frame.copy()

    frame["relative_time_seconds"] = pd.to_numeric(
        frame["relative_time_seconds"],
        errors="coerce",
    )

    frame["value"] = pd.to_numeric(
        frame["value"],
        errors="coerce",
    )

    frame = frame.dropna(
        subset=[
            "relative_time_seconds",
            "value",
            "metric",
            "experiment_name",
        ]
    )

    if frame.empty:
        raise ValueError(
            "CSV does not contain any valid numeric samples."
        )

    # The first Prometheus sample can have a relative time such as
    # -0.0002 because of timestamp precision.
    nearly_zero = frame["relative_time_seconds"].between(
        -0.01,
        0,
        inclusive="both",
    )
    frame.loc[nearly_zero, "relative_time_seconds"] = 0.0

    frame["experiment_name"] = (
        frame["experiment_name"].astype(str)
    )
    frame["experiment_id"] = (
        frame["experiment_id"].astype(str)
    )
    frame["metric"] = frame["metric"].astype(str)

    frame["series"] = (
        frame["series"]
        .fillna("all")
        .astype(str)
    )

    # The immediate parent folder identifies a specific experiment run.
    frame["_run_folder"] = csv_path.parent.name
    frame["_source_file"] = str(csv_path)

    return frame


def load_all_results(
    metrics_files: list[Path],
) -> pd.DataFrame:
    loaded_frames: list[pd.DataFrame] = []

    for csv_path in metrics_files:
        try:
            frame = load_metrics_file(csv_path)
        except ValueError as error:
            print(
                f"Warning: skipping {csv_path}",
                file=sys.stderr,
            )
            print(
                f"  Reason: {error}",
                file=sys.stderr,
            )
            continue

        loaded_frames.append(frame)

        experiment_names = sorted(
            frame["experiment_name"].unique()
        )

        print(
            f"Loaded: {csv_path} "
            f"({', '.join(experiment_names)})"
        )

    if not loaded_frames:
        raise RuntimeError(
            "No valid metrics.csv file was loaded."
        )

    return pd.concat(
        loaded_frames,
        ignore_index=True,
    )


def add_display_names(frame: pd.DataFrame) -> pd.DataFrame:
    frame = frame.copy()

    runs = (
        frame[
            [
                "experiment_id",
                "experiment_name",
                "_run_folder",
            ]
        ]
        .drop_duplicates()
    )

    name_counts = (
        runs.groupby("experiment_name")
        .size()
        .to_dict()
    )

    display_names: dict[str, str] = {}

    for experiment_id, experiment_name, run_folder in runs.itertuples(
        index=False,
        name=None,
    ):
        experiment_id = str(experiment_id)
        experiment_name = str(experiment_name)
        run_folder = str(run_folder)

        if name_counts[experiment_name] == 1:
            display_name = experiment_name
        else:
            # Add the run folder when multiple runs have the same name.
            display_name = (
                f"{experiment_name} [{run_folder}]"
            )

        display_names[experiment_id] = display_name

    frame["_display_name"] = (
        frame["experiment_id"]
        .map(display_names)
        .fillna(frame["experiment_name"])
    )

    return frame


def ordered_metrics(frame: pd.DataFrame) -> list[str]:
    available = set(frame["metric"].unique())

    distributed_lock_metrics = {
        "distributed_lock_successful",
        "distributed_lock_failed",
    }
    if available & distributed_lock_metrics:
        available -= distributed_lock_metrics
        available.add("distributed_lock_attempts")

    known_metrics = [
        metric
        for metric in METRIC_ORDER
        if metric in available
    ]

    unknown_metrics = sorted(
        available - set(METRIC_ORDER)
    )

    return known_metrics + unknown_metrics


def metric_title(metric: str) -> str:
    if metric in METRIC_TITLES:
        return METRIC_TITLES[metric]

    return metric.replace("_", " ").strip().title()


def metric_unit(metric: str) -> str:
    return METRIC_UNITS.get(metric, "Вредност")

def format_series_name(series: str) -> str:
    series_names = {
        "level=l1": "L1",
        "level=l2": "L2",
    }

    return series_names.get(series, series)

def build_line_label(
    display_name: str,
    series: str,
    has_multiple_series: bool,
) -> str:
    if not has_multiple_series and series == "all":
        return display_name

    if series == "all":
        return display_name

    formatted_series = format_series_name(series)

    return f"{display_name} — {formatted_series}"


def configure_axis(
    axis: plt.Axes,
    metric: str,
) -> None:
    axis.set_title(
        metric_title(metric),
        fontsize=12,
        fontweight="bold",
    )

    axis.set_xlabel("Време од почетка експеримента (s)")
    axis.set_ylabel(metric_unit(metric))

    axis.grid(
        visible=True,
        which="major",
        linestyle="--",
        linewidth=0.6,
        alpha=0.55,
    )

    axis.margins(x=0.01)

    formatter = ScalarFormatter(useMathText=True)
    formatter.set_powerlimits((-3, 4))
    if metric == "adaptive_ttl_state":
        axis.set_ylim(-0.15, 3.15)
        axis.set_yticks([0, 1, 2, 3])
        axis.set_yticklabels(
            [
                "Disabled",
                "Stable",
                "Warning",
                "Congested",
            ]
        )
    axis.yaxis.set_major_formatter(formatter)


def plot_metric(
    axis: plt.Axes,
    frame: pd.DataFrame,
    metric: str,
) -> None:
    if metric == "distributed_lock_attempts":
        metric_frame = frame[
            frame["metric"].isin(
                {
                    "distributed_lock_successful",
                    "distributed_lock_failed",
                }
            )
        ].copy()
    else:
        metric_frame = frame[
            frame["metric"] == metric
        ].copy()

    distinct_series = set(metric_frame["series"])
    has_multiple_series = (
        len(distinct_series) > 1
        or distinct_series != {"all"}
    )

    group_columns = [
        "experiment_id",
        "_display_name",
        "series",
    ]
    if metric == "distributed_lock_attempts":
        group_columns.append("metric")

    experiment_colors: dict[str, str] = {}
    labeled_faf_experiments: set[str] = set()
    if metric == "faf_by_key":
        color_cycle = plt.rcParams["axes.prop_cycle"].by_key()["color"]
        experiment_ids = sorted(
            metric_frame["experiment_id"].unique()
        )
        experiment_colors = {
            experiment_id: color_cycle[index % len(color_cycle)]
            for index, experiment_id in enumerate(experiment_ids)
        }

    labeled_lock_metrics: set[str] = set()

    for group_key, group in metric_frame.groupby(
        group_columns,
        sort=True,
    ):
        experiment_id = group_key[0]
        display_name = group_key[1]
        series = group_key[2]
        lock_metric = (
            group_key[3]
            if metric == "distributed_lock_attempts"
            else ""
        )

        group = group.sort_values(
            "relative_time_seconds"
        )

        # Average duplicate values recorded for the same timestamp.
        group = (
            group.groupby(
                "relative_time_seconds",
                as_index=False,
            )["value"]
            .mean()
        )

        if metric == "distributed_lock_attempts":
            lock_labels = {
                "distributed_lock_successful": "Успешни",
                "distributed_lock_failed": "Неуспешни",
            }
            label = "_nolegend_"
            if lock_metric not in labeled_lock_metrics:
                label = lock_labels[lock_metric]
                labeled_lock_metrics.add(lock_metric)
        elif metric == "faf_by_key":
            label = "_nolegend_"
            if experiment_id not in labeled_faf_experiments:
                label = str(display_name)
                labeled_faf_experiments.add(experiment_id)
        else:
            label = build_line_label(
                display_name=str(display_name),
                series=str(series),
                has_multiple_series=has_multiple_series,
            )

        plot_options = {
            "linewidth": 1.6,
            "label": label,
        }
        
        step_metrics = {
            "adaptive_ttl_state",
            "adaptive_ttl_factor",
            "adaptive_ttl_effective_seconds",
        }

        if metric in step_metrics:
            plot_options["drawstyle"] = "steps-post"

        if metric == "faf_by_key":
            plot_options["color"] = experiment_colors[experiment_id]
        elif metric == "distributed_lock_attempts":
            lock_colors = {
                "distributed_lock_successful": "tab:green",
                "distributed_lock_failed": "tab:red",
            }
            plot_options["color"] = lock_colors[lock_metric]

        axis.plot(
            group["relative_time_seconds"],
            group["value"],
            **plot_options,
        )

    configure_axis(axis, metric)

    handles, labels = axis.get_legend_handles_labels()

    if handles:
        legend_columns = 1
        if len(handles) > 8:
            legend_columns = 2

        axis.legend(
            handles,
            labels,
            fontsize=7.5,
            loc="best",
            framealpha=0.9,
            ncol=legend_columns,
        )


def create_comparison_image(
    frame: pd.DataFrame,
    metrics: list[str],
    output_path: Path,
    columns: int,
    dpi: int,
    title: str,
) -> None:
    rows = math.ceil(len(metrics) / columns)

    figure_width = 8.0 * columns
    figure_height = 3.8 * rows + 1.0

    figure, axes = plt.subplots(
        nrows=rows,
        ncols=columns,
        figsize=(figure_width, figure_height),
        squeeze=False,
    )

    flattened_axes = axes.flatten()

    for axis, metric in zip(flattened_axes, metrics):
        plot_metric(
            axis=axis,
            frame=frame,
            metric=metric,
        )

    # Hide unused subplots.
    for axis in flattened_axes[len(metrics):]:
        axis.set_visible(False)

    experiment_count = frame[
        "experiment_id"
    ].nunique()

    figure.suptitle(
        (
            f"{title}\n"
            f"Број експеримената: {experiment_count}"
        ),
        fontsize=17,
        fontweight="bold",
    )

    figure.tight_layout(
        rect=(0, 0, 1, 0.965)
    )

    figure.savefig(
        output_path,
        format="jpg",
        dpi=dpi,
        bbox_inches="tight",
        facecolor="white",
        pil_kwargs={
            "quality": 95,
            "optimize": True,
        },
    )

    plt.close(figure)


def determine_output_path(
    results_folder: Path,
    output_argument: str,
) -> Path:
    output_path = Path(output_argument)

    if not output_path.is_absolute():
        output_path = results_folder / output_path

    output_path = output_path.resolve()

    if output_path.suffix.lower() not in {
        ".jpg",
        ".jpeg",
    }:
        output_path = output_path.with_suffix(".jpg")

    output_path.parent.mkdir(
        parents=True,
        exist_ok=True,
    )

    return output_path


def main() -> int:
    args = parse_arguments()

    try:
        if args.columns <= 0:
            raise ValueError(
                "--columns must be greater than zero."
            )

        if args.dpi <= 0:
            raise ValueError(
                "--dpi must be greater than zero."
            )

        results_folder = resolve_results_folder(
            args.results_folder
        )

        metrics_files = find_metrics_files(
            results_folder
        )

        if not metrics_files:
            raise FileNotFoundError(
                "No metrics.csv file was found in folder: "
                f"{results_folder}"
            )

        print(
            f"CSV files found: {len(metrics_files)}"
        )

        combined_frame = load_all_results(
            metrics_files
        )

        combined_frame = add_display_names(
            combined_frame
        )

        metrics = ordered_metrics(
            combined_frame
        )

        if not metrics:
            raise RuntimeError(
                "No metrics were found."
            )

        output_path = determine_output_path(
            results_folder=results_folder,
            output_argument=args.output,
        )

        title = (
            args.title
            if args.title
            else f"Компаративни резултати — {results_folder.name}"
        )

        print(
            f"Experiments: "
            f"{combined_frame['experiment_id'].nunique()}"
        )
        print(f"Metrics: {len(metrics)}")
        print("Generating charts...")

        create_comparison_image(
            frame=combined_frame,
            metrics=metrics,
            output_path=output_path,
            columns=args.columns,
            dpi=args.dpi,
            title=title,
        )

        print()
        print("Image created successfully:")
        print(output_path)

        return 0

    except Exception as error:
        print(
            f"Error: {error}",
            file=sys.stderr,
        )
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
