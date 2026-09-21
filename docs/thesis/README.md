# Thesis document

Source of the written thesis: outline with page budgets, figure register, evaluation protocol,
reference list and the AI-use declaration. Tooling (LaTeX via Overleaf, or Word) is decided
once the supervisor answers how he wants drafts; nothing here depends on it. Text is written
as the work happens: one page plus a figure per week from the chapter this week touched.

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
