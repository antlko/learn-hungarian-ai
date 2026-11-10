/** The one place the published image name lives. `:latest` has Python (components install from the app); `:latest-slim` is distroless without it. */
export const DEPLOY_IMAGE = 'ghcr.io/antlko/learn-hungarian-ai:latest'

export const DOCKER_RUN = `docker run -d --name tanulom --restart unless-stopped -p 8012:8012 -v tanulom-data:/data ${DEPLOY_IMAGE}`

export const COMPOSE_YAML = `services:
  tanulom:
    image: ${DEPLOY_IMAGE}
    ports:
      - '8012:8012'
    volumes:
      - tanulom-data:/data
    restart: unless-stopped

volumes:
  tanulom-data:`
