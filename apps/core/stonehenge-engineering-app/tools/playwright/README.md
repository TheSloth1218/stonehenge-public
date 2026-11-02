# Playwright Tooling

Containerized browser validation for the Engineering Console.

The top-level compose file includes a `playwright` profile that mounts this directory at `/work` and points `BASE_URL` at the engineering app container.

Typical use from the monorepo root:

```sh
docker compose --profile tools run --rm playwright npx playwright test
```

Use this for layout checks, iframe/FUXA rendering checks, and auth/API boundary smoke tests.

Run `tests/training-profiles.spec.js` twice against separately recreated safe
and training app profiles. Set `TRAINING_PROFILE=safe` or `training` in the
Playwright container to select the expected authorization assertions.
