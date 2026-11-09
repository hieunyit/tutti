# Optional build contexts with the repository docs and the OpenAPI spec, bundled into the
# Documentation and API reference pages. Pass them with
#   docker build --build-context docs=docs --build-context proto=proto -f ui/Dockerfile ui
# (docker-compose does this). Without them these empty stages are used and the pages say
# the content is not part of this build.
FROM scratch AS docs
FROM scratch AS proto

# Build stage
FROM --platform=$BUILDPLATFORM node:20 as build-stage
# The UI imports ../docs and ../proto relative to the repository root, so place them at
# /docs and /proto next to /app.
COPY --from=docs / /docs/
COPY --from=proto / /proto/
WORKDIR /app

# Install dependencies
COPY package.json yarn.lock ./
ENV NODE_ENV=development
RUN yarn install --pure-lockfile
COPY . .
# Build
ENV NODE_ENV=production
RUN yarn build

# Production stage
FROM nginx:stable-alpine as production-stage
COPY --from=build-stage /app/build /usr/share/nginx/html
COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template

ARG SERVICE
ARG BUILD_DATE
ARG GIT_COMMIT
ARG GIT_TAG
ENV GIT_COMMIT=$GIT_COMMIT
ENV GIT_TAG=$GIT_TAG
ENV BUILD_DATE=$BUILD_DATE

# when run with docker, provide chorus worker management api URL in envar:
# docker run -p 80:80 -e BACKEND_URL=http://127.0.0.1:9090

