# Contributing Guidelines

Thank you for your interest in contributing to **BeamSync**! We welcome contributions from the community. By participating, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

1. **Fork the repository**
   - Click the *Fork* button on the top right of the repository page.
2. **Clone your fork**
   ```bash
   git clone https://github.com/<your-username>/BeamSync.git
   cd BeamSync
   ```
3. **Create a new branch**
   ```bash
   git checkout -b <feature-or-fix-name>
   ```
4. **Make your changes**
   - Follow the existing code style and linting rules.
   - Ensure any new code is covered by tests where applicable.
5. **Commit your changes**
   ```bash
   git add .
   git commit -m "Brief description of your change"
   ```
6. **Push to your fork**
   ```bash
   git push origin <feature-or-fix-name>
   ```
7. **Open a Pull Request**
   - Navigate to the original repository and click *New Pull Request*.
   - Fill out the PR template and provide a clear description of what you have done.

## Code Style
- Go code should be formatted with `gofmt` and pass `go vet`.
- JavaScript/TypeScript should follow the project's ESLint configuration.
- Svelte components should use the project's Prettier settings.

## Testing
- Run unit tests with `go test ./...` for Go packages.
- Front‑end tests can be run with `npm test` inside `desktop/frontend`.
- Ensure all tests pass before submitting a PR.

## Documentation
- Update the `README.md` or relevant docs if your change modifies public behavior.
- Keep changelog entries in `CHANGELOG.md` (if present).

## Reporting Issues
- If you encounter a bug, please open an issue using the appropriate template.

## Thank You!
Your contributions help make BeamSync better for everyone.
