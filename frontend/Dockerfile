FROM node:22

WORKDIR /app

COPY frontend/package*.json ./

RUN npm install

COPY frontend .

EXPOSE 4200

CMD ["npx", "ng", "serve", "--host", "0.0.0.0", "--port", "4200", "--poll", "2000"]