FROM node:22.22-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts
COPY server.js ./
COPY src ./src
USER node
EXPOSE 8092
CMD ["node", "server.js"]
