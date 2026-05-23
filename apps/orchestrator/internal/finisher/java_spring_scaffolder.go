// JavaSpringScaffolder — Spring Boot 3.3.x skeleton (Java 21, Maven,
// JPA, Postgres). Mirrors the GameScaffolder / EcommerceScaffolder
// shape: deterministic files + a contract markdown the Coder reads as
// part of project context.
//
// Triggers when the spec mentions Java, Spring, Kotlin-on-server, or
// stories describe "spring boot" / "enterprise" workloads.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type JavaSpringScaffolder struct{}

func (JavaSpringScaffolder) Name() string { return "java-spring" }

func (JavaSpringScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "java") || strings.Contains(stack, "spring") {
		return true
	}
	// Kotlin can also target the JVM as a Spring backend.
	if strings.Contains(stack, "kotlin") && strings.Contains(stack, "server") {
		return true
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "spring boot") || strings.Contains(desc, "enterprise") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "spring boot") || strings.Contains(body, "enterprise") {
			return true
		}
	}
	return false
}

func (JavaSpringScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<!--
  Spring Boot 3.3.x + Java 21. Keep the parent in sync with the BOM that
  spring-boot-starter-parent already pulls in — do NOT pin individual
  Spring/Hibernate versions, the parent manages them transitively.
-->
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0
                             https://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>

  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>3.3.4</version>
    <relativePath/>
  </parent>

  <groupId>com.example</groupId>
  <artifactId>app</artifactId>
  <version>0.1.0</version>
  <packaging>jar</packaging>
  <name>Ironflyer Spring App</name>

  <properties>
    <java.version>21</java.version>
    <maven.compiler.source>21</maven.compiler.source>
    <maven.compiler.target>21</maven.compiler.target>
    <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
  </properties>

  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-data-jpa</artifactId>
    </dependency>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-validation</artifactId>
    </dependency>
    <dependency>
      <groupId>org.postgresql</groupId>
      <artifactId>postgresql</artifactId>
      <scope>runtime</scope>
    </dependency>
  </dependencies>

  <build>
    <plugins>
      <plugin>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-maven-plugin</artifactId>
      </plugin>
    </plugins>
  </build>
</project>
`,
		"src/main/java/com/example/app/Application.java": `// Spring Boot entrypoint. Component scanning picks up controllers,
// services, and JPA repositories under com.example.app.*.
package com.example.app;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class Application {
    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }
}
`,
		"src/main/java/com/example/app/controller/HealthController.java": `// Liveness + version endpoints. Wire these into your platform health
// check (k8s probe, Render healthcheck, etc.) — they are intentionally
// unauthenticated so probes work before the security chain initialises.
package com.example.app.controller;

import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class HealthController {

    @GetMapping("/health")
    public Map<String, String> health() {
        return Map.of("status", "ok");
    }

    @GetMapping("/version")
    public Map<String, String> version() {
        return Map.of("name", "ironflyer-spring-app", "version", "0.1.0");
    }
}
`,
		"src/main/java/com/example/app/controller/UserController.java": `// Example CRUD controller. Replace User with your real domain model
// once the spec is concrete — this scaffold exists so the JPA wiring
// is testable end-to-end out of the box.
package com.example.app.controller;

import com.example.app.model.User;
import com.example.app.repo.UserRepository;
import jakarta.validation.Valid;
import java.util.List;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/users")
public class UserController {

    private final UserRepository repo;

    public UserController(UserRepository repo) {
        this.repo = repo;
    }

    @GetMapping
    public List<User> list() {
        return repo.findAll();
    }

    @GetMapping("/{id}")
    public ResponseEntity<User> get(@PathVariable Long id) {
        return repo.findById(id)
            .map(ResponseEntity::ok)
            .orElseGet(() -> ResponseEntity.notFound().build());
    }

    @PostMapping
    public User create(@Valid @RequestBody User user) {
        return repo.save(user);
    }

    @PutMapping("/{id}")
    public ResponseEntity<User> update(@PathVariable Long id, @Valid @RequestBody User patch) {
        return repo.findById(id).map(existing -> {
            existing.setEmail(patch.getEmail());
            existing.setName(patch.getName());
            return ResponseEntity.ok(repo.save(existing));
        }).orElseGet(() -> ResponseEntity.notFound().build());
    }

    @DeleteMapping("/{id}")
    public ResponseEntity<Void> delete(@PathVariable Long id) {
        if (!repo.existsById(id)) {
            return ResponseEntity.notFound().build();
        }
        repo.deleteById(id);
        return ResponseEntity.noContent().build();
    }
}
`,
		"src/main/java/com/example/app/model/User.java": `// JPA entity. ddl-auto: update keeps the schema in sync during dev;
// switch to a Flyway migration before going to production.
package com.example.app.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;

@Entity
@Table(name = "users")
public class User {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @NotBlank
    @Column(nullable = false)
    private String name;

    @Email
    @NotBlank
    @Column(nullable = false, unique = true)
    private String email;

    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }

    public String getName() { return name; }
    public void setName(String name) { this.name = name; }

    public String getEmail() { return email; }
    public void setEmail(String email) { this.email = email; }
}
`,
		"src/main/java/com/example/app/repo/UserRepository.java": `// Spring Data JPA gives us findAll / findById / save / deleteById for
// free — extend this interface when you need custom queries.
package com.example.app.repo;

import com.example.app.model.User;
import org.springframework.data.jpa.repository.JpaRepository;

public interface UserRepository extends JpaRepository<User, Long> {
}
`,
		"src/main/resources/application.yml": `# Spring config. DATABASE_URL is the canonical Postgres connection string
# (e.g. jdbc:postgresql://host:5432/db?user=...&password=...). ddl-auto:
# update is fine for the scaffold — swap for Flyway/Liquibase before prod.
spring:
  datasource:
    url: ${DATABASE_URL}
  jpa:
    hibernate:
      ddl-auto: update
    properties:
      hibernate:
        dialect: org.hibernate.dialect.PostgreSQLDialect
        format_sql: true
  jackson:
    serialization:
      indent_output: true

server:
  port: ${PORT:8080}

management:
  endpoints:
    web:
      exposure:
        include: health,info
`,
		"Dockerfile": `# Multi-stage build: Maven on Temurin 21 to produce the fat jar, then
# the slim Temurin 21 JRE for runtime. The final image is ~250MB and
# starts in <5s on a warm JVM.
FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /workspace
COPY pom.xml .
RUN mvn -B -q dependency:go-offline
COPY src ./src
RUN mvn -B -q -DskipTests package

FROM eclipse-temurin:21-jre-jammy
WORKDIR /app
COPY --from=build /workspace/target/*.jar app.jar
EXPOSE 8080
ENTRYPOINT ["java","-jar","/app/app.jar"]
`,
		".gitignore": `# Maven
target/
!.mvn/wrapper/maven-wrapper.jar

# IDE
.idea/
*.iml
.vscode/
.classpath
.project
.settings/

# OS
.DS_Store
Thumbs.db

# Env
.env
.env.local
`,
	}
	contract := `Java Spring Boot scaffold: Spring Boot 3.3.x, Java 21, Maven, JPA, Postgres.

Already provisioned:
- /pom.xml                                                → Maven config, Spring Boot parent BOM
- /src/main/java/com/example/app/Application.java         → @SpringBootApplication entrypoint
- /src/main/java/com/example/app/controller/HealthController.java → /health + /version
- /src/main/java/com/example/app/controller/UserController.java   → CRUD on /api/users
- /src/main/java/com/example/app/model/User.java          → JPA @Entity example
- /src/main/java/com/example/app/repo/UserRepository.java → extends JpaRepository<User, Long>
- /src/main/resources/application.yml                     → datasource from DATABASE_URL
- /Dockerfile                                             → maven:3.9-temurin-21 build → temurin-21-jre runtime

Contract for the Coder:
1. Run with: mvn spring-boot:run
2. DATABASE_URL is REQUIRED (jdbc:postgresql://host:5432/db?user=...&password=...).
3. spring.jpa.hibernate.ddl-auto: update handles the initial schema —
   switch to Flyway/Liquibase migrations before promoting to production.
4. Add new controllers under com.example.app.controller, entities under
   .model, repositories under .repo. Spring's component scan finds them.
5. PORT env var overrides the default 8080 (Render/Heroku-friendly).
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
