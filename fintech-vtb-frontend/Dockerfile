# Собираем React + Vite внутри Docker — Node.js/npm на хосте не нужны.
FROM node:22-alpine AS build

WORKDIR /app

COPY package.json ./
RUN npm install

COPY . .

# Значение "/" после удаления завершающего слеша становится пустым base URL,
# поэтому браузер обращается к текущему origin, а Nginx проксирует /api/.
ARG VITE_API_BASE_URL=/
ARG VITE_USE_MOCK_DATA=false
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_USE_MOCK_DATA=${VITE_USE_MOCK_DATA}

RUN npm run build

FROM nginx:alpine AS runtime

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1
