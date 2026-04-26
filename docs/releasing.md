# Releasing

The project uses GitHub Actions to automate the release process. A workflow is triggered whenever a tag starting with `v` is pushed to the repository.

## Process

1.  **Tag the release**:
    Create a new tag locally and push it to GitHub.

    ```bash
    git tag v0.1.0
    git push origin v0.1.0
    ```

2.  **Wait for the build**:
    The [Release workflow](../.github/workflows/release.yml) will automatically start. It performs the following steps:
    -   Builds the binary for macOS `arm64` (Apple Silicon) and `amd64` (Intel).
    -   Combines them into a single universal binary using `lipo`.
    -   Archives the binary into a `.tar.gz` file.
    -   Generates a SHA-256 checksum for the archive.

3.  **Review and Publish**:
    Once the workflow completes, a new **Draft Release** will be created on GitHub.
    -   Go to the [Releases page](https://github.com/codcod/maints-triage/releases).
    -   Edit the draft release to review the auto-generated release notes.
    -   Publish the release when ready.

## Artifacts

The release will include:
-   `maints-vX.Y.Z-darwin-universal.tar.gz`: The universal macOS binary.
-   `maints-vX.Y.Z-darwin-universal.tar.gz.sha256`: The checksum file.
