# GitHub Automation

GitHub Actions are intentionally disabled for this repository.

Run `make ci` locally for validation. Run `make release`, `make publish`, and `make deploy` only from a clean local `master` branch that exactly matches `origin/master` with zero open pull requests. The production scripts verify and print that Git state before release, publish, or deploy work begins.
