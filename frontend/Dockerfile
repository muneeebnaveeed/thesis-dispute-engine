# syntax=docker/dockerfile:1
# Multi-stage: install with the pnpm store as a cache mount, build, run the Nitro node-server output.
# NODE_VERSION and PNPM_VERSION must match mise.toml; CI passes them in.
ARG NODE_VERSION=24.21.0
ARG PNPM_VERSION=12.5.1

FROM node:${NODE_VERSION}-alpine AS build
ARG PNPM_VERSION
RUN npm install -g pnpm@${PNPM_VERSION}
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
# Dependencies first so the fetch layer survives source changes; the store mount survives everything.
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm config set store-dir /root/.local/share/pnpm/store && pnpm fetch
COPY . .
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --offline --frozen-lockfile --ignore-scripts && pnpm generate-routes && pnpm build

FROM node:${NODE_VERSION}-alpine
ENV NODE_ENV=production PORT=3002
WORKDIR /app
COPY --from=build /src/.output ./.output
EXPOSE 3002
USER node
CMD ["node", ".output/server/index.mjs"]
