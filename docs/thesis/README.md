# Thesis document

Source of the written thesis: outline with page budgets, figure register, evaluation protocol,
reference list, the AI-use declaration, and the LaTeX project itself under `latex/` (decided:
Overleaf). Text is written as the work happens: one page plus a figure per week from the chapter
this week touched, into the chapter file under `latex/chapters/`.

## Building and Overleaf

`make thesis` builds `latex/build/main.pdf` with TeX Live in Docker (no local TeX needed);
`latexmk -pdf main.tex` inside `latex/` does the same with a local installation. The repository is the
source of truth. Overleaf shows and compiles the same files in one of two ways:

- Overleaf's Git integration (a premium feature, included in some university licences): add the
  project's git URL as a remote and push the `latex/` directory as a subtree,
  `git subtree push --prefix docs/thesis/latex overleaf master`; pull edits back with
  `git subtree pull`. Edits made in Overleaf come back through a PR like any other change.
- Without a premium account: upload `latex/` as a zip to a new project for the supervisor's
  Grammarly pass, and apply his comments here. The PDF for him is `make thesis` either way.

Formatting is fixed in `latex/main.tex`: 12 pt Times-compatible text, 1.5 spacing, margins
3/2/3/3 cm, numbered figures and tables, listings numbered as figures, `\code{}` for identifiers
so the final uniform pass is one macro. `references.bib` mirrors `references.md`.

Formal requirements (inf.unideb.hu/node/486): Times New Roman 12 pt, 1.5 spacing, margins
3/2/3/3 cm (L/R/T/B), 30 to 40 pages of main text, appendix at most 8 to 10 pages, faculty
title page, auto-generated table of contents, no lists of figures or tables. Supervisor's
guidance: at least 20 to 25 pages on the application and its development, at most 10
references, figures at most a third of the document, every figure and table numbered and
referenced in the text (a code snippet is a figure), identifiers set like code in a final
uniform pass, AI use declared. Grading: literature review, documentation of the work, applied
techniques and results, professional standard including spelling and formatting.

| File | What |
| --- | --- |
| `outline.md` | Chapters, page budgets, what each draws on |
| `figures.md` | Figure register: number, caption, source, status |
| `evaluation.md` | What is measured, how, and where the numbers land (`make eval`) |
| `references.md` | The reference list, capped at ten |
| `ai-declaration.md` | The declaration text, kept true as the work goes |
| `data/` | Evaluation runs, one dated directory each |
| `figures/` | Exported figures (diagrams, screenshots), named by figure number |
