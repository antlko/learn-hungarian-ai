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

/** The one place the repository URL lives. */
export const REPO_URL = 'https://github.com/antlko/learn-hungarian-ai'

/** The app's own run snippets, written once so the sidecar ones can extend them. */
const APP_SERVICE = `  tanulom:
    image: ${DEPLOY_IMAGE}
    ports:
      - '8012:8012'
    volumes:
      - tanulom-data:/data
    restart: unless-stopped`

/**
 * An optional service that can run beside the app from the SAME image
 * (`component run <id>`). Ports are the services' own defaults
 * (backend/internal/components, services/<id>/app.py); the model ids are
 * what those app.py files download.
 */
export interface Sidecar {
  id: 'mt'
  model: string
  licence: string
  modelUrl: string
  sourceUrl: string
  sourceLabel: string
  port: number
  serviceName: string
}

export const SIDECARS: Record<string, Sidecar> = {
  mt: {
    id: 'mt',
    model: 'facebook/nllb-200-distilled-600M',
    licence: 'CC BY-NC 4.0',
    modelUrl: 'https://huggingface.co/facebook/nllb-200-distilled-600M',
    sourceUrl: `${REPO_URL}/tree/main/services/mt`,
    sourceLabel: 'services/mt',
    port: 5003,
    serviceName: 'tanulom-mt',
  },
}

export const sidecarEndpoint = (s: Sidecar) => `http://${s.serviceName}:${s.port}`

export const sidecarCompose = (s: Sidecar) => `services:
${APP_SERVICE}

  ${s.serviceName}:
    image: ${DEPLOY_IMAGE}
    command: ["component", "run", "${s.id}"]
    expose:
      - '${s.port}'
    volumes:
      - ${s.serviceName}:/data
    restart: unless-stopped

volumes:
  tanulom-data:
  ${s.serviceName}:`

export const sidecarDocker = (s: Sidecar) =>
  [
    'docker network create tanulom',
    `docker run -d --name tanulom --network tanulom --restart unless-stopped -p 8012:8012 -v tanulom-data:/data ${DEPLOY_IMAGE}`,
    `docker run -d --name ${s.serviceName} --network tanulom --restart unless-stopped -v ${s.serviceName}:/data ${DEPLOY_IMAGE} component run ${s.id}`,
  ].join('\n')
