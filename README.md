# FindVibe

One repository containing the Go/Fiber API in `backend/` and the Angular app in
`frontend/`. This repository starts with a clean history; the original repositories
linked below retain their full histories.

## Local development

Requires Go 1.26+, Node.js compatible with Angular 21 (Node 24 LTS recommended),
npm, Docker Desktop with its engine running, and Air on your PATH:

```sh
go install github.com/air-verse/air@v1.61.7
./start.sh
```

The launcher works from any directory on macOS. On the first run it installs
frontend dependencies, copies `backend/.env.example` to `backend/.env`, and
starts an isolated local PostgreSQL database. Go dependencies download as needed.
Air rebuilds the API on Go changes; Angular reloads on frontend changes.
Logs from both apps appear together with labels.

- Frontend: http://localhost:4200
- API health: http://localhost:8081/health
- PostgreSQL: `127.0.0.1:55432`, database `findvibe`, user/password `postgres`

Ctrl-C stops both apps and removes the database container, retaining its named
volume. Docker Desktop stays running. Occupied ports cause an error rather than
stopping another project. Startup errors return a nonzero exit status.

Search, discovery and recommendations require a Last.fm key. Add `LASTFM_API_KEY` to `backend/.env` and
restart. Secrets are ignored by Git. The launcher explicitly sets local database
and API settings; it does not use a production database.

After frontend dependency changes, run `cd frontend && npm ci`.

## Checks

```sh
(cd backend && go test ./...)
(cd frontend && npm run build)
(cd frontend && npm run lint)
```

## Repository origins and deployment

- `backend/`: https://github.com/andiq123/FindVibeFiber at `8781a21`
- `frontend/`: https://github.com/andiq123/FindVibeFrontEnd at `7900bf8`

These are ordinary directories, not submodules or nested repositories. The Go
module path is retained so existing imports continue to work. Deployment services
must use `backend/` or `frontend/` as their root directory when moved to this repo.
Vercel uses root `frontend`, build command `npm run build`, and output directory
`dist/client-angular/browser`. Set `API_URL=https://find-vibe.firewifi.online`.
The production build generates its environment from `API_URL`; local development
keeps using localhost. The Pi service uses root `backend`, entrypoint `./cmd`,
and its existing linked PostgreSQL database and service environment variables.
