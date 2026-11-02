FROM mcr.microsoft.com/playwright:v1.45.3-jammy

WORKDIR /work

COPY package.json ./
RUN npm install

COPY . ./
