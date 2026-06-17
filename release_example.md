
1. **Create an annotated tag** on the commit you want to release.

   ```sh
   git tag -a v0.1.1 -m "release v0.1.1"
   ```

2. **Push the tag.** This is what triggers the workflow — a regular `git push`
   does **not** push tags.

   ```sh
   git push origin v0.1.1
   ```
   
```sh
git tag -d v0.1.1
git push origin :refs/tags/v0.1.1