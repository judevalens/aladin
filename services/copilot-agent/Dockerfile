FROM node:20-alpine

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm install --omit=dev --no-audit --no-fund

COPY server.js ./
COPY src/ ./src/

EXPOSE 3550

CMD ["node", "server.js"]
