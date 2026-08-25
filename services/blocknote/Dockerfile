FROM node:22-alpine

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install --omit=dev --no-audit --no-fund

COPY server.js ./
COPY src/ ./src/

EXPOSE 3500

CMD ["node", "server.js"]
