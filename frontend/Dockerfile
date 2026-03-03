# Build stage
FROM node:20-alpine as build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npm run build

# Serve stage
FROM nginx:1.25-alpine
COPY --from=build /app/dist /usr/share/nginx/html

# Copy the entrypoint script
COPY entrypoint.sh /
RUN chmod +x /entrypoint.sh

EXPOSE 80

# Start Nginx using the entrypoint script
ENTRYPOINT ["/entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
