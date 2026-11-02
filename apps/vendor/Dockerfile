FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /src
COPY pom.xml .
RUN mvn -q -DskipTests dependency:go-offline
COPY src ./src
RUN mvn -q -DskipTests package

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /src/target/sca-parts-fulfillment-0.1.0-SNAPSHOT.jar app.jar
RUN groupadd --system stonevendor && useradd --system --gid stonevendor --home-dir /app stonevendor
USER stonevendor
EXPOSE 8089
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
