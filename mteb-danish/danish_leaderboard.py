"""Reproduce the MTEB "Danish" leaderboard from the public results repository.

MTEB has no Danish-only benchmark. The leaderboard page shows "MTEB(Scandinavian, v1)"
and lets you filter by language; this script does exactly what the leaderboard does
(same mteb code path, Borda ranking, revision joining) restricted to the `dan` rows.

Usage:
    pip install "mteb>=2.20" polars pandas
    git clone --depth 1 https://github.com/embeddings-benchmark/results mteb-results
    python danish_leaderboard.py mteb-results

Writes danish_leaderboard.csv (Borda rank, means, per-task scores, all models).
"""

import logging
import sys
import warnings

warnings.filterwarnings("ignore")
logging.disable(logging.CRITICAL)

import mteb  # noqa: E402
import pandas as pd  # noqa: E402
import polars as pl  # noqa: E402
from mteb.cache import ResultCache  # noqa: E402

results_dir = sys.argv[1] if len(sys.argv) > 1 else "mteb-results"
bench = mteb.get_benchmark("MTEB(Scandinavian, v1)")
cache = ResultCache(cache_path=results_dir)
br = cache.load_results(tasks=bench, include_remote=False)

# One row per (model, task, split, language subset, score); revisions joined as on the leaderboard.
long = br._to_results_df(bench.tasks)
dan = long.filter(
    pl.col("language").list.eval(pl.element().str.split("-").list.first() == "dan").list.any()
)

# Summary table exactly as the leaderboard builds it (Borda rank, Mean(Task), Mean(TaskType), ...).
summary = bench._create_summary_table(dan).df.to_pandas()
summary = summary[["Rank (Borda)", "Model", "Zero-shot", "Total Parameters (B)", "Mean (Task)", "Mean (TaskType)"]]

# Per-task scores (mean over Danish subsets and splits), scaled to 0-100.
per_task = (
    dan.group_by(["model_name", "task_name"]).agg(pl.col("score").mean()).to_pandas()
    .pivot(index="model_name", columns="task_name", values="score")
    .mul(100).round(2).reset_index().rename(columns={"model_name": "Model"})
)
out = summary.merge(per_task, on="Model", how="left")
out["Mean (Task)"] = (out["Mean (Task)"] * 100).round(2)
out["Mean (TaskType)"] = (out["Mean (TaskType)"] * 100).round(2)
out.to_csv("danish_leaderboard.csv", index=False)
print(out.head(20).to_string())
