# Getting to the top of the MTEB Danish embedding leaderboard

Question: what would it take to get to the top of the MTEB Danish language
embedding leaderboard? What do the existing models score, and what were they
trained on?

Short answer: the "Danish leaderboard" is the `MTEB(Scandinavian, v1)`
benchmark filtered to its 12 Danish tasks. As of the results snapshot of
2026-09-07 it is led by three general multilingual models that were never
built for Danish: **codefuse-ai/F2LLM-v2-14B** (Danish mean 75.9),
**Qwen/Qwen3-Embedding-8B** (74.6) and **google/gemini-embedding-001** (72.8).
The Borda ranking that decides the order is tight (2169 vs 2151 vs 2144
points). Every model that was purpose-built for Danish or Scandinavian sits at
rank 11 or lower, and the two published attempts to fine-tune a strong base
model on the largest open Scandinavian training set made the base model
*worse* by 7 to 13 points. Simulations on the actual results show that a
Qwen3-Embedding-8B that gained 2 points on every Danish task, or that matched
the best model on a single high-variance task (TwitterHjerne retrieval), would
take first place. The realistic route is therefore: start from
Qwen3-Embedding-8B or F2LLM-v2-14B, continue training with a *mixture* of the
existing synthetic Nordic data (about 2.1M Danish/Norwegian/Swedish pairs are
public) and the original multilingual mix, target the six tasks that actually
separate the top models, and validate against the numbers below before
submitting. Details and caveats follow.

Everything here was computed from the public
[embeddings-benchmark/results](https://github.com/embeddings-benchmark/results)
repository (commit `476d40d`, 2026-09-07) with the same `mteb` (2.20.11) code
path the Hugging Face leaderboard uses, so the numbers should match what the
leaderboard shows with the language filter set to Danish. The full table for
all 237 models is in [`danish_leaderboard.csv`](danish_leaderboard.csv) and
[`danish_leaderboard.py`](danish_leaderboard.py) reproduces it.

## What "the MTEB Danish leaderboard" actually is

There is no Danish-only benchmark in MTEB. The [leaderboard](https://huggingface.co/spaces/mteb/leaderboard)
offers `MTEB(Scandinavian, v1)`, which is the [Scandinavian Embedding
Benchmark (SEB)](https://arxiv.org/abs/2406.02396) by Enevoldsen, Kardos,
Muennighoff and Nielbo (NeurIPS 2024), merged into MTEB in 2025. The SEB site
now redirects to MTEB. The benchmark has 28 tasks; the language filter reduces
it to the 12 that contain Danish:

| Task | Type | Metric | What it is | Test size |
|---|---|---|---|---|
| AngryTweetsClassification | Classification | accuracy | 3-class sentiment on Danish tweets | 1,047 |
| DanishPoliticalCommentsClassification | Classification | accuracy | Sentiment on political comments (train split is the eval split) | 7,206 |
| DKHateClassification | Classification | accuracy | Offensive / not offensive tweets | 329 |
| LccSentimentClassification | Classification | accuracy | Sentiment on Leipzig corpus sentences | 150 |
| MassiveIntentClassification (da) | Classification | accuracy | Amazon MASSIVE, 60 intents, Danish subset | 2,974 |
| MassiveScenarioClassification (da) | Classification | accuracy | Amazon MASSIVE, 18 scenarios, Danish subset | 2,974 |
| NordicLangClassification | Classification | accuracy | Identify da/sv/nb/nn/is/fo | 3,000 |
| ScalaClassification (da) | Classification | accuracy | Linguistic acceptability, binary (ScandEval) | 8,192 |
| DanFeverRetrieval | Retrieval | nDCG@10 | Claim to Wikipedia evidence (DanFEVER) | 8,897 |
| TV2Nordretrieval | Retrieval | nDCG@10 | News summary to article (TV2 Nord) | 4,096 |
| TwitterHjerneRetrieval | Retrieval | nDCG@10 | #Twitterhjerne question to answer | 340 |
| BornholmBitextMining | Bitext mining | F1 | Bornholmsk dialect to Danish | 500 |

Classification tasks in MTEB are scored by training a logistic regression on
the model's embeddings of the train split, so they measure how linearly
separable the embedding space is, not zero-shot labelling.

How the ordering works, from the leaderboard code in `mteb/leaderboard/`:

- The default sort is **Rank (Borda)**: for every task, a model earns one
  point per model it beats. Missing a task earns zero points on that task, so
  a model must run all 12 Danish tasks to compete. "Mean (Task)" and
  "Mean (TaskType)" are shown but do not decide the rank.
- The **Zero-shot** filter defaults to "Allow All". A model that declares
  MTEB evaluation datasets in its training data is shown with a lower
  zero-shot percentage but is not removed. F2LLM-v2 (83%) trained on the
  MASSIVE train splits; KaLM-Embedding-Gemma3-12B (58%) and the Harrier
  models trained on Bornholm, NordicLang and ScaLA themselves. Switching the
  filter to "Only Zero-shot" makes Qwen3-Embedding-8B first.
- Scores for a multilingual task are the mean over the selected language
  subsets, and revisions of the same model are merged.

## Current standings (Danish filter, 2026-09-07)

Top of the table. Means are in percent; "ZS" is the zero-shot percentage.

| Rank | Model | Params | ZS | Mean (Task) | Mean (TaskType) | Classif. | Retrieval | Bitext |
|---|---|---|---|---|---|---|---|---|
| 1 | codefuse-ai/F2LLM-v2-14B | 14.0B | 83 | 75.9 | 69.6 | 79.1 | 74.1 | 55.6 |
| 2 | Qwen/Qwen3-Embedding-8B | 7.6B | 100 | 74.6 | 74.4 | 75.8 | 70.9 | 76.3 |
| 3 | google/gemini-embedding-001 | API | 100 | 72.8 | 67.9 | 73.1 | 78.9 | 51.7 |
| 4 | codefuse-ai/F2LLM-v2-8B | 7.6B | 83 | 74.2 | 67.0 | 77.6 | 72.9 | 50.6 |
| 5 | codefuse-ai/F2LLM-v2-4B | 4.0B | 83 | 72.4 | 65.7 | 75.5 | 71.6 | 50.0 |
| 6 | Qwen/Qwen3-Embedding-4B | 4.0B | 100 | 72.5 | 71.3 | 74.1 | 68.7 | 71.3 |
| 7 | codefuse-ai/F2LLM-v2-1.7B | 1.7B | 83 | 71.2 | 64.9 | 74.2 | 70.3 | 50.2 |
| 8 | Salesforce/SFR-Embedding-2_R | 7.1B | 100 | 66.4 | 62.7 | 66.7 | 70.5 | 50.8 |
| 9 | GritLM/GritLM-7B | 7.2B | 100 | 63.4 | 62.0 | 65.1 | 59.6 | 61.2 |
| 10 | codefuse-ai/F2LLM-v2-0.6B | 0.6B | 83 | 67.6 | 61.0 | 70.6 | 66.9 | 45.7 |
| 11 | nicher92/saga-embed_v1 | 0.4B | 100 | 65.2 | 57.9 | 66.7 | 71.0 | 36.0 |
| 12 | openai/text-embedding-3-large | API | ? | 63.8 | 59.7 | 63.1 | 72.5 | 43.6 |
| 13 | Cohere/Cohere-embed-multilingual-v3.0 | API | ? | 62.6 | 56.5 | 62.7 | 71.2 | 35.8 |
| 16 | voyageai/voyage-multilingual-2 | API | 100 | 61.7 | 59.0 | 59.0 | 75.4 | 42.7 |
| 19 | Qwen/Qwen3-Embedding-0.6B | 0.6B | 100 | 62.1 | 59.7 | 63.3 | 61.7 | 54.2 |
| 24 | intfloat/multilingual-e5-large-instruct | 0.56B | 100 | 59.1 | 57.6 | 60.2 | 57.2 | 55.4 |
| 33 | BAAI/bge-m3 | 0.57B | 100 | 56.3 | 53.0 | 57.4 | 57.6 | 44.1 |
| 40 | KennethEnevoldsen/dfm-sentence-encoder-large | 0.36B | 100 | 56.0 | 43.8 | 61.9 | 53.4 | 15.9 |
| 46 | emillykkejensen/EmbeddingGemma-Scandi-300m | 0.31B | 100 | 55.4 | 49.6 | 57.4 | 56.9 | 34.4 |
| 52 | emillykkejensen/mmBERTscandi-base-embedding | 0.31B | 100 | 53.8 | 43.2 | 58.9 | 51.8 | 18.9 |
| 64 | emillykkejensen/Qwen3-Embedding-Scandi-0.6B | 0.6B | 100 | 49.5 | 42.5 | 53.0 | 47.5 | 27.1 |

Only 59 of the 237 listed models have run all 12 Danish tasks. Notable
absentees: Gemini Embedding 2 (April 2026) has no Danish results except one
preview run on Bornholm; KaLM-Embedding-Gemma3-12B-2511 (the MMTEB leader in
late 2025) has only 5 of 12 and trained on three of them; jina-embeddings-v3
is missing DanFever; the Tech Collective's TTC-L2V-supervised-2 was never
submitted to MTEB at all (see below).

Per-task scores for the models that matter, in percent:

| Task | F2LLM-v2-14B | Qwen3-8B | gemini-001 | Qwen3-4B | saga-embed | text-emb-3-large | mE5-large-instruct | dfm-large |
|---|---|---|---|---|---|---|---|---|
| AngryTweets | 68.3 | **69.8** | 64.1 | 66.7 | 63.4 | 57.9 | 59.5 | 54.5 |
| Bornholm bitext | 55.6 | **76.3** | 51.7 | 71.3 | 36.0 | 43.6 | 55.4 | 15.9 |
| DKHate | 84.6 | 85.9 | **87.0** | 84.5 | 73.7 | 70.2 | 64.5 | 63.3 |
| DanFever | 38.6 | 39.7 | **41.7** | 39.8 | 38.9 | 39.9 | 40.6 | 37.2 |
| DaPoliticalComments | 52.0 | 53.1 | 52.1 | **54.6** | 46.2 | 43.4 | 33.1 | 37.8 |
| LCC sentiment | 75.9 | **76.7** | 69.9 | **76.7** | 63.9 | 58.0 | 60.3 | 57.5 |
| MASSIVE intent | **89.1** | 82.7 | 84.4 | 81.5 | 69.2 | 69.0 | 65.3 | 66.3 |
| MASSIVE scenario | **94.1** | 87.4 | 89.3 | 85.8 | 73.7 | 75.6 | 71.9 | 71.1 |
| NordicLang | 85.2 | **92.0** | 86.0 | 90.8 | 91.1 | 79.7 | 76.6 | 76.0 |
| ScaLA | **83.9** | 59.0 | 52.0 | 51.9 | 52.4 | 50.8 | 50.5 | 68.8 |
| TV2Nord retrieval | 96.3 | 94.9 | **97.1** | 93.6 | 95.9 | 95.2 | 94.1 | 80.8 |
| TwitterHjerne | 87.2 | 78.2 | **98.0** | 72.6 | 78.2 | 82.5 | 36.9 | 42.3 |
| **Mean** | **75.9** | 74.6 | 72.8 | 72.5 | 65.2 | 63.8 | 59.1 | 56.0 |

Two things stand out. First, the winner does not win most tasks: F2LLM-v2-14B
is best on only three of the twelve (the two MASSIVE tasks it trained on, and
ScaLA), Qwen3-8B on three or four, Gemini on three. Second, the tasks fall into two
groups. DanFever (all models 38 to 42) and TV2Nord (the top ten 93 to 98)
are saturated and contribute almost nothing to the ordering. The leverage is
in Bornholm (spread 36 to 76 among the top 20), ScaLA (51 to 84), TwitterHjerne
(43 to 98), NordicLang (49 to 92), the two MASSIVE tasks (67 to 89, 60 to 94)
and DKHate (65 to 87).

## The models at the top and what they were trained on

**codefuse-ai/F2LLM-v2-14B** (Ant Group, March 2026, Apache 2.0, [paper](https://arxiv.org/abs/2603.19223)).
Qwen3-14B-Base turned into an embedding model with two-stage contrastive
training on a fully released dataset,
[codefuse-ai/F2LLM-v2](https://huggingface.co/datasets/codefuse-ai/F2LLM-v2):
60M examples from 157 public sources covering 282 languages, chosen "by
real-world data availability rather than optimizing for specific benchmarks".
Stage 1 is 27M pairs from seven large retrieval sources (mMARCO, WebFAQ,
CLIRMatrix, ParaCrawl, CodeSearchNet, ...), stage 2 is 18M mixed samples
capped at 80K queries per source with task instructions, hard negatives mined
with Qwen3-Embedding-8B, Matryoshka training, batch 512, learning rate 5e-6, 2
epochs. The interesting part for Danish: its declared training sets include
four synthetic Nordic corpora published by SEB co-author Márton Kardos
(`kardosdrur/synthetic-nordic-retrieval`, `-classification`, `-sts`,
`-text_matching`, see the data section), plus the MASSIVE intent and scenario
train splits, which is why it is 83% zero-shot and why it wins those two
tasks by 6 to 7 points. The paper reports first place on the full
`MTEB(Scandinavian)` benchmark with 71.10 over 28 tasks. The whole size
ladder (0.6B, 1.7B, 4B, 8B, 14B) lands in the Danish top 10, so the data
recipe matters more than the parameter count.

**Qwen/Qwen3-Embedding-8B** (Alibaba, June 2025, Apache 2.0, [report](https://arxiv.org/abs/2506.05176)).
Qwen3-8B with a two-stage pipeline: roughly 150M synthetic pairs generated by
Qwen3-32B across retrieval, bitext mining, STS and classification with
persona, difficulty and language sampled per example; then about 7M labelled
pairs (MS MARCO, NQ, HotpotQA, NLI, MIRACL, Mr.TyDi, MLDR, DuReader,
CodeSearchNet, ...) plus 12M filtered synthetic pairs, and finally slerp
merging of checkpoints. No Danish-specific data is documented. It is the best
fully zero-shot model, the only top model that handles the Bornholmsk dialect
(76.3 vs 56 or less for everyone else), and it is 18 Borda points behind the
leader. The 4B version is rank 6 and 0.6B is rank 19.

**google/gemini-embedding-001** (Google, GA July 2025, API only, [paper](https://arxiv.org/abs/2503.07891)).
Initialised from Gemini with bidirectional attention, pre-finetuned on a
billion-scale web corpus of title/passage pairs, then fine-tuned on academic
datasets, synthetic retrieval data (few-shot prompted queries over web
passages, filtered by a Gemini auto-rater) and synthetic English
classification data, with a model soup at the end. The paper says it excluded
many in-domain MTEB datasets. Best retrieval model on the Danish tasks (98.0
on TwitterHjerne, best on DanFever and TV2Nord) and best on DKHate, but weak
on Bornholm and ScaLA. Gemini Embedding 2 (May 2026 paper, 69.9 on MTEB
multilingual) has not been run on the Danish tasks.

**nicher92/saga-embed_v1** (May 2026, MIT). The best purpose-built
Scandinavian model, rank 11 with 0.4B parameters, "the highest ranked model
under 1.5B". A ModernBERT-base architecture (via AI Sweden) trained on about
250M semantically related pairs and then fine-tuned with task prompts. The
model card promises a technical report and gives no dataset list, so the 250M
pairs are not verifiable. Strong on NordicLang (91.1) and retrieval, weak on
Bornholm (36.0).

**intfloat/multilingual-e5-large-instruct** (Microsoft, 2024, [report](https://arxiv.org/abs/2402.05672)).
XLM-RoBERTa-large, contrastively pretrained on about 1B multilingual pairs
(mC4 160M, CC News 160M, NLLB 160M, Reddit 160M, Wikipedia 150M, xP3 80M,
S2ORC 50M, StackExchange 50M) and fine-tuned on about 1.6M labelled examples
(MS MARCO 570K, NLI 275K, NQ/TriviaQA/SQuAD 220K, ELI5, NLLB, FEVER,
HotpotQA, Mr.TyDi, MIRACL, DuReader, Quora) plus the E5-mistral synthetic
data. Danish arrives only through the web-scale pretraining. This was the
best open model when SEB was published (Danish 61.1 in the paper) and the
model the Danish Foundation Models group still lists as state of the art;
today it is rank 24.

**jealk/TTC-L2V-supervised-2** (The Tech Collective, May 2025, MIT). The
model behind the "world's best Scandinavian AI model" and "#1 for Danish,
Swedish and Norwegian" claims. AI-Sweden's Llama-3-8B-instruct converted with
the LLM2Vec recipe: bidirectional attention, masked next token prediction on
`jealk/scandi-wiki-combined` (1.4M Scandinavian Wikipedia rows), supervised
SimCSE on `jealk/supervised-da` (148K Danish query/positive pairs: 30K news,
29K Gemma-generated Wikipedia queries, 10K Europarl, 10K OpenSubtitles, 9K
WikiQA, 4K Folketinget, 2K Hestenet), then supervised SimCSE with hard
negatives and instructions on `DDSC/nordic-embedding-training-data`. Trained
on a single A100 in about 24 hours per the company's write-up. It was added to
the SEB repository in release v0.13.11 (2025-05-17) as "new SOTA", which was
true against the May 2025 field, one month before Qwen3-Embedding and two
before gemini-embedding-001 shipped. It was never submitted to the MTEB
results repository, so it does not appear on the leaderboard at all. From the
SEB repository's cached run, its Danish mean over the 12 tasks is 68.4
(TwitterHjerne 85.0, LCC 73.7, AngryTweets 67.1, ScaLA 53.0, Bornholm 54.6).
Under MTEB's harness that would land around rank 8 to 10 today, with the
caveat that SEB and MTEB runs differ for some models (SEB's own run of
mE5-large-instruct scores 77.2 on TwitterHjerne where MTEB's scores 36.9, so
prompt handling differs).

**emillykkejensen/*-Scandi** (October 2025, Apache 2.0). Three
straightforward Sentence-Transformers fine-tunes on the DDSC data with
CachedMultipleNegativesRankingLoss, batch 16 to 64, learning rate 5e-6, one
epoch: EmbeddingGemma-300m, Qwen3-Embedding-0.6B and mmBERT-base. These are
the clearest data points on what naive Danish fine-tuning does, because the
base models are on the leaderboard too:

| Base | Base mean | Fine-tuned mean | Tasks compared |
|---|---|---|---|
| google/embeddinggemma-300m | 62.7 | 55.6 | 6 shared tasks |
| Qwen/Qwen3-Embedding-0.6B | 62.1 | 49.5 | all 12 |

The Qwen3-0.6B fine-tune lost 27 points on Bornholm, 20 on LCC, 17 on
TV2Nord and 20 on TwitterHjerne. One epoch of in-language triplets at a tiny
batch size, with no replay of the original training mix, no instructions
matching the base model's format, and LoRA merged into a model that was itself
produced by checkpoint merging, wrecks what made the base good.

**KennethEnevoldsen/dfm-sentence-encoder-large** (SEB baseline). DanskBERT
(`chcaa/dfm-encoder-large-v1`) with unsupervised SimCSE on Danish Gigaword
paragraphs, one epoch, batch 128, max length 32. Rank 40; interesting only
because it gets 68.8 on ScaLA, the second best clean score, from a
Danish-pretrained encoder with no contrastive supervision.

**Contaminated high scores to ignore**: `tencent/KaLM-Embedding-Gemma3-12B-2511`
(86.2 ScaLA, 93.8 NordicLang, 90.4 TwitterHjerne) and `harrier-oss-v1-27b`
(90.0 ScaLA, 95.8 NordicLang) list BornholmBitextMining, NordicLangClassification
and ScalaClassification among their training sets, and neither ran the full
Danish set.

## The Danish training data that exists

The table below is what is public. Sizes are rows; "Danish" is the Danish
share where the dataset is split by language.

| Dataset | Rows | Danish | What it is | Generated by | License |
|---|---|---|---|---|---|
| [DDSC/nordic-embedding-training-data](https://huggingface.co/datasets/DDSC/nordic-embedding-training-data) ("NordicE5") | 968K | 484K (no 243K, sv 242K) | query/positive/hard-negative triplets with instructions, five task types (retrieval 186K, classification 196K, short text-matching 199K, long text-matching 190K, unit-triple 198K) | Gemma-2-27b-it, following the E5-mistral synthetic recipe ([arXiv 2401.00368](https://arxiv.org/abs/2401.00368)); compute from Arrow Denmark and Nvidia via Danish Data Science Community | none stated |
| [kardosdrur/synthetic-nordic-retrieval](https://huggingface.co/datasets/kardosdrur/synthetic-nordic-retrieval) | 190K | 95K | query, positive, hard negative, instruction | unspecified LLM, same recipe | none stated |
| [kardosdrur/synthetic-nordic-classification](https://huggingface.co/datasets/kardosdrur/synthetic-nordic-classification) | 199K | 100K | text, label, misleading label, instruction | same | none stated |
| [kardosdrur/synthetic-nordic-sts](https://huggingface.co/datasets/kardosdrur/synthetic-nordic-sts) | 396K | 198K | sentence pairs with similarity 0.5 to 0.9 | same | none stated |
| [kardosdrur/synthetic-nordic-text_matching](https://huggingface.co/datasets/kardosdrur/synthetic-nordic-text_matching) | 385K | 193K | input, positive, instruction | same | none stated |
| [jealk/supervised-da](https://huggingface.co/datasets/jealk/supervised-da) | 148K | all | real Danish query/passage pairs (news, Europarl, Folketinget, OpenSubtitles, WikiQA, Hestenet) plus 29K Gemma-generated Wikipedia queries | mixed | none stated |
| [jealk/scandi-wiki-combined](https://huggingface.co/datasets/jealk/scandi-wiki-combined) | 1.4M | mixed | Wikipedia articles and sentences, da/no/sv/is/fo | Wikipedia | CC BY-SA (inherited) |
| [danish-foundation-models/danish-dynaword](https://huggingface.co/datasets/danish-foundation-models/danish-dynaword) | 7.4M docs, 9.8B tokens | all | Openly licensed Danish text: parliament records 2.8B tokens, EU Cellar 1.2B, historical newspapers 1.0B, KB publications, legal texts, books, the Hest forum, ... | human | CC-0 collection, per-source CC-0 / CC-BY / CC-BY-SA |
| MTEB task train splits | 2.4K to 57K each | all | AngryTweets, DKHate, LCC, MASSIVE, NordicLang, ScaLA train splits | human | various |

So there are roughly 2.1M synthetic Danish/Norwegian/Swedish pairs already
generated (about 1.07M Danish), all in the same E5-mistral style, all
produced by 27B-class open models, and none with a stated license. F2LLM-v2
already consumed the kardosdrur sets. Nobody has yet published a model that
mixes these with the strong base models' own recipe rather than replacing it.
There is no Danish retrieval data with human-written queries at scale; the
closest are the TV2 Nord summaries (75K training pairs in
`alexandrainst/nordjylland-news-summarization`, CC0, but its test split *is*
the TV2Nord eval task, so only the train split is usable) and the small QA
sources in `jealk/supervised-da`.

## What it would take to reach rank 1

### The target, in numbers

Borda points with 237 models and 12 tasks:

| Model | Borda | Mean (Task) |
|---|---|---|
| F2LLM-v2-14B | 2169 | 75.9 |
| Qwen3-Embedding-8B | 2151 | 74.6 |
| gemini-embedding-001 | 2144 | 72.8 |

I re-ranked hypothetical models against the real table:

| Hypothetical model | Rank | Borda | Mean |
|---|---|---|---|
| Qwen3-8B unchanged | 2 | 2151 | 74.6 |
| Qwen3-8B with F2LLM's ScaLA score | 2 | 2163 | 76.7 |
| Qwen3-8B with F2LLM's two MASSIVE scores | 2 | 2168 | 75.7 |
| Qwen3-8B with Gemini's TwitterHjerne score | **1** | 2179 | 76.3 |
| Qwen3-8B plus 2 points on every task | **1** | 2215 | 76.6 |
| F2LLM-14B with Qwen3's Bornholm score | **1** | 2182 | 77.6 |
| Best of Qwen3-8B and F2LLM-14B per task | **1** | 2212 | 78.7 |

The margin is small enough that a single-task improvement on a
high-variance task flips first place, and a uniform 2-point gain over
Qwen3-8B does it with room to spare. A Danish mean around 76 to 77 with no
task below the current leaders is the bar. Note that because the rank is
Borda, a mean of 76.7 (the ScaLA scenario) can still be rank 2: winning one
task big is worth the same as winning it narrowly.

### Where the points are

- **TwitterHjerne retrieval** (340 questions): Gemini scores 98.0, Qwen3-8B
  78.2, the field spans 43 to 98. Short conversational Danish questions to
  answers. Nothing in the public synthetic data looks like this; the Twitter
  domain and question-to-answer asymmetry are the gap. Generating a few tens
  of thousands of Danish social-media style question/answer pairs is cheap.
- **ScaLA** (8,192 sentences): binary grammatical acceptability, so 50 is
  chance and most models sit there. F2LLM reaches 83.9 without training on
  ScaLA, DanskBERT-based dfm-sentence-encoder reaches 68.8. This rewards
  Danish-specific *token-level* pretraining and hard negatives that differ by
  word order or morphology. Synthetic "corrupted sentence" negatives from
  Dynaword text would target it directly.
- **Bornholm bitext mining**: Qwen3-8B 76.3, everyone else at most 61. The
  Bornholmsk parallel corpus is tiny and dialectal; a base model that knows
  Danish orthographic variation wins. Avoid losing this: it is where the
  naive fine-tunes lost 20 to 27 points.
- **MASSIVE intent and scenario**: F2LLM leads by 6 to 7 points because it
  trained on the train splits, which the leaderboard permits and flags.
  Doing the same costs nothing and closes the gap; not doing it and still
  matching F2LLM would require intent-style synthetic data.
- **NordicLang, DKHate, AngryTweets, LCC, political comments**: 1 to 7 point
  spreads among the top models. Classification-style synthetic data (the
  DDSC and kardosdrur classification sets) is the obvious lever, and the
  MTEB train splits are usable.
- **DanFever and TV2Nord**: ignore. Every serious model gets 38 to 42 and 93
  to 98 respectively. DanFever's ceiling looks like a property of the dataset
  rather than of the models, since everything from 0.4B to 14B lands in the
  same 4-point band.

### A concrete plan

1. **Pick the base.** Qwen3-Embedding-8B (Apache 2.0, best zero-shot, best
   on Bornholm and NordicLang) or F2LLM-v2-14B (Apache 2.0, fully open data
   and code, already includes the kardosdrur Nordic sets). Qwen3-8B is the
   better starting point if you want a zero-shot #1; F2LLM-14B if you want
   the highest absolute score. Starting below 4B is not competitive: the
   best sub-1B model is rank 10 at 67.6.
2. **Do not replace the recipe, extend it.** The two failed fine-tunes both
   trained on Nordic triplets alone. Continue training with a mixture:
   roughly 10 to 30 percent Nordic data (DDSC + kardosdrur + supervised-da,
   deduplicated, with hard negatives re-mined using the base model itself so
   they are actually hard), the rest from the base model's public stage-2
   style data (for F2LLM the 18M stage-2 mix is downloadable; for Qwen3 use
   the public supervised sets it lists: MS MARCO, NQ, HotpotQA, NLI, MIRACL,
   Mr.TyDi, MLDR). Keep the base model's instruction format on the query
   side. Use a large effective batch (thousands, via GradCache), a low
   learning rate (1e-6 to 5e-6, the numbers both Qwen3 and F2LLM report),
   and stop early against a held-out slice of the Danish tasks. Consider
   slerp-merging the fine-tuned checkpoint back with the base, which both
   Qwen3 and Gemini do and which is the cheapest way to avoid the 10-point
   regressions seen above.
3. **Generate the missing task shapes.** Danish question-to-answer pairs in
   a social register (TwitterHjerne), acceptability pairs with minimal edits
   (ScaLA), intent-style short commands (MASSIVE), and dialect/spelling
   variation pairs (Bornholm). Seed the generation from Dynaword (9.8B
   tokens, CC-0 or CC-BY, which also answers the license question the
   existing synthetic sets leave open) and from the MASSIVE and ScaLA train
   splits. A 27B or larger open model following the E5-mistral prompts is
   enough; the existing sets were made with Gemma-2-27b. Budget: a few
   hundred thousand pairs, which is a day of generation on one 80GB GPU.
4. **Decide the zero-shot question up front.** Training on MTEB train
   splits (MASSIVE, DKHate, AngryTweets, ScaLA, NordicLang) is allowed and
   the current leader does it, but you must declare it in the model's
   `ModelMeta.training_datasets`, and the model will show below 100% zero-shot.
   Training on eval splits, or on the TV2 Nord test split, is contamination.
5. **Evaluate exactly as the leaderboard does before submitting.** Run
   `mteb` on `MTEB(Scandinavian, v1)` (all 28 tasks, or you are not ranked on
   the benchmark page), compare per task against the table above, and check
   Swedish and Norwegian did not collapse. Submission is a pull request to
   `embeddings-benchmark/results` with the result JSONs and a `ModelMeta`
   entry in `mteb`; the harness documentation is at
   https://embeddings-benchmark.github.io/mteb/.
6. **Compute.** LoRA or full fine-tuning of an 8B model on 1 to 3M pairs
   with large batches is on the order of 1 to 3 days on 8 H100s, or a week
   on a single node of 4 A100s with GradCache; the 14B model roughly
   doubles that. Evaluation of the 28 tasks takes a few GPU hours per
   checkpoint. This is well within what the Tech Collective spent (one A100
   day) times ten, and far below what any of the top three models cost.

### What would not work

- Fine-tuning a sub-1B encoder, however Danish: saga-embed is the best of
  that class at rank 11, 10 points behind the leader.
- Training only on the existing synthetic Nordic triplets. Demonstrated
  twice to lose 7 to 13 points against the base.
- Optimising for Mean (Task) alone. The rank is Borda; a big win on one task
  and small losses on three others can lower the rank.
- Skipping tasks. A missing task scores zero Borda points; KaLM-12B and
  Gemini Embedding 2 are unranked for this reason.

## Caveats

- The leaderboard is a moving target. This snapshot is 2026-09-07; F2LLM-v2
  took first place in March 2026 and saga-embed appeared in May 2026. Gemini
  Embedding 2, Qwen3-VL-Embedding, jina-embeddings-v5 and Seed 1.6 exist but
  have not run the Danish tasks.
- Numbers from the SEB repository (used for TTC-L2V-2 only) and from the
  MTEB results repository are not interchangeable; the same model can differ
  by tens of points on TwitterHjerne between the two harnesses.
- The Zero-shot column depends on self-declared metadata. A model that
  trained on Danish Wikipedia queries derived from the same articles as
  DanFever is still "zero-shot".
- None of the synthetic Nordic datasets state a license, and the DDSC set
  was generated with Gemma 2, whose terms apply to derived data. Dynaword is
  the only large Danish corpus with clean licensing.
- The Danish tasks are small (150 to 8,192 items; TwitterHjerne has 340
  queries), so differences of 1 to 2 points on a single task are within
  noise, even though they move the Borda rank.

## Reproducing

```sh
pip install "mteb>=2.20" polars pandas
git clone --depth 1 https://github.com/embeddings-benchmark/results mteb-results   # ~3.8 GB checked out
python danish_leaderboard.py mteb-results        # writes danish_leaderboard.csv
```

The script loads the results the way the leaderboard does
(`ResultCache.load_results` for the `MTEB(Scandinavian, v1)` benchmark,
`_to_results_df` to join revisions, a filter on `dan` language codes, and the
benchmark's own `_create_summary_table` for the Borda rank). The hypothetical
re-rankings above were done by appending a row to that per-task matrix and
recomputing Borda points as the sum over tasks of models strictly beaten.

## Sources

- MTEB leaderboard: https://huggingface.co/spaces/mteb/leaderboard and the
  `mteb` package (2.20.11), `mteb/leaderboard/app.py`, `mteb/benchmarks/_create_table.py`
- Results repository: https://github.com/embeddings-benchmark/results (commit 476d40d, 2026-09-07)
- SEB paper: Enevoldsen et al., "The Scandinavian Embedding Benchmarks", NeurIPS 2024, https://arxiv.org/abs/2406.02396; repository https://github.com/KennethEnevoldsen/scandinavian-embedding-benchmark (release notes and `src/seb/cache/`)
- F2LLM-v2: https://arxiv.org/abs/2603.19223, https://huggingface.co/codefuse-ai/F2LLM-v2-14B, https://huggingface.co/datasets/codefuse-ai/F2LLM-v2
- Qwen3 Embedding: https://arxiv.org/abs/2506.05176, https://huggingface.co/Qwen/Qwen3-Embedding-8B
- Gemini Embedding: https://arxiv.org/abs/2503.07891; Gemini Embedding 2: https://arxiv.org/abs/2605.27295
- Multilingual E5: https://arxiv.org/abs/2402.05672, https://huggingface.co/intfloat/multilingual-e5-large-instruct
- saga-embed: https://huggingface.co/nicher92/saga-embed_v1
- TTC-L2V: https://huggingface.co/jealk/TTC-L2V-supervised-2, https://huggingface.co/jealk/TTC-L2V-supervised-1, https://thetechcollective.eu/insights/creating-the-worlds-best-model-for-scandinavian-languages, https://thetechcollective.eu/insights/how-to-train-a-top-5-nordic-ai-model
- emillykkejensen models: https://huggingface.co/emillykkejensen/Qwen3-Embedding-Scandi-0.6B, https://huggingface.co/emillykkejensen/EmbeddingGemma-Scandi-300m, https://huggingface.co/emillykkejensen/mmBERTscandi-base-embedding
- dfm-sentence-encoder: https://huggingface.co/KennethEnevoldsen/dfm-sentence-encoder-large; DFM model collection https://huggingface.co/collections/danish-foundation-models/state-of-the-art-danish-models
- KaLM-Embedding-Gemma3-12B: https://huggingface.co/tencent/KaLM-Embedding-Gemma3-12B-2511
- Datasets: https://huggingface.co/datasets/DDSC/nordic-embedding-training-data, the four `kardosdrur/synthetic-nordic-*` sets, https://huggingface.co/datasets/jealk/supervised-da, https://huggingface.co/datasets/jealk/scandi-wiki-combined, https://huggingface.co/datasets/danish-foundation-models/danish-dynaword, https://huggingface.co/datasets/alexandrainst/nordjylland-news-summarization, https://huggingface.co/datasets/strombergnlp/danfever
- Synthetic data recipe: Wang et al., "Improving Text Embeddings with Large Language Models", https://arxiv.org/abs/2401.00368
