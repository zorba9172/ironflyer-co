// DotNetScaffolder — ASP.NET Core 9 minimal-API baseline. Triggers
// when the planner picks .NET / ASP.NET / C#, or when a user story
// names ASP.NET Core. Emits a single-project Web SDK app with EF
// Core + Npgsql, swagger in dev, and a /users CRUD group bound to a
// minimal-API endpoint mapping.
//
// The Dockerfile is a stock multi-stage build over the official
// dotnet/sdk + dotnet/aspnet images. Migrations are NOT generated
// upfront — they are created by `dotnet ef migrations add Init`
// once the developer is happy with the model surface.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type DotNetScaffolder struct{}

func (DotNetScaffolder) Name() string { return "dotnet-aspnet" }

func (DotNetScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, ".net") || strings.Contains(stack, "aspnet") ||
		strings.Contains(stack, "asp.net") || strings.Contains(stack, "c#") ||
		strings.Contains(stack, "csharp") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "asp.net core") {
			return true
		}
	}
	return false
}

func (DotNetScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"App.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web">

  <PropertyGroup>
    <TargetFramework>net9.0</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <RootNamespace>App</RootNamespace>
    <AssemblyName>App</AssemblyName>
    <InvariantGlobalization>true</InvariantGlobalization>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.EntityFrameworkCore.Design" Version="9.0.0">
      <PrivateAssets>all</PrivateAssets>
      <IncludeAssets>runtime; build; native; contentfiles; analyzers; buildtransitive</IncludeAssets>
    </PackageReference>
    <PackageReference Include="Npgsql.EntityFrameworkCore.PostgreSQL" Version="9.0.2" />
    <PackageReference Include="Swashbuckle.AspNetCore" Version="6.9.0" />
  </ItemGroup>

</Project>
`,
		"Program.cs": `// ASP.NET Core 9 minimal API. The whole app boots from this file:
//   1. Bind config + DI.
//   2. Register EF Core against DATABASE_URL (Heroku-style postgres
//      URLs are translated into an Npgsql connection string).
//   3. Add swagger in development.
//   4. Map /health and /users CRUD.
using App.Data;
using App.Models;
using Microsoft.EntityFrameworkCore;

var builder = WebApplication.CreateBuilder(args);

string connectionString = ResolveConnectionString(builder.Configuration);

builder.Services.AddDbContext<AppDbContext>(opt => opt.UseNpgsql(connectionString));
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

var app = builder.Build();

if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

app.MapGet("/health", () => Results.Ok(new { status = "ok" }));

var users = app.MapGroup("/users");

users.MapGet("/", async (AppDbContext db) => await db.Users.ToListAsync());

users.MapGet("/{id:int}", async (int id, AppDbContext db) =>
    await db.Users.FindAsync(id) is User u ? Results.Ok(u) : Results.NotFound());

users.MapPost("/", async (User input, AppDbContext db) =>
{
    db.Users.Add(input);
    await db.SaveChangesAsync();
    return Results.Created($"/users/{input.Id}", input);
});

users.MapPut("/{id:int}", async (int id, User input, AppDbContext db) =>
{
    var existing = await db.Users.FindAsync(id);
    if (existing is null) return Results.NotFound();
    existing.Email = input.Email;
    existing.Name = input.Name;
    await db.SaveChangesAsync();
    return Results.Ok(existing);
});

users.MapDelete("/{id:int}", async (int id, AppDbContext db) =>
{
    var existing = await db.Users.FindAsync(id);
    if (existing is null) return Results.NotFound();
    db.Users.Remove(existing);
    await db.SaveChangesAsync();
    return Results.NoContent();
});

app.Run();

// Translates a postgres:// URL (the platform's preferred shape) into
// an Npgsql key=value connection string. Falls back to the named
// connection string in appsettings.json when DATABASE_URL is unset.
static string ResolveConnectionString(IConfiguration config)
{
    var url = Environment.GetEnvironmentVariable("DATABASE_URL");
    if (string.IsNullOrWhiteSpace(url))
    {
        return config.GetConnectionString("Default") ?? "Host=localhost;Database=app;Username=postgres;Password=";
    }
    var uri = new Uri(url);
    var userInfo = uri.UserInfo.Split(':', 2);
    var user = userInfo[0];
    var pass = userInfo.Length > 1 ? userInfo[1] : string.Empty;
    return $"Host={uri.Host};Port={(uri.Port > 0 ? uri.Port : 5432)};Database={uri.AbsolutePath.TrimStart('/')};Username={user};Password={pass};SSL Mode=Prefer;Trust Server Certificate=true";
}
`,
		"Models/User.cs": `// User POCO mapped by EF Core. Keep this lean — validation
// attributes and computed columns can be added as the model
// grows.
namespace App.Models;

public class User
{
    public int Id { get; set; }
    public string Email { get; set; } = string.Empty;
    public string? Name { get; set; }
    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
}
`,
		"Data/AppDbContext.cs": `// EF Core context. One DbSet per aggregate; configuration lives
// in OnModelCreating so we can tighten constraints without
// reaching for attributes on the POCO.
using App.Models;
using Microsoft.EntityFrameworkCore;

namespace App.Data;

public class AppDbContext : DbContext
{
    public AppDbContext(DbContextOptions<AppDbContext> options) : base(options) {}

    public DbSet<User> Users => Set<User>();

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        modelBuilder.Entity<User>(entity =>
        {
            entity.HasIndex(u => u.Email).IsUnique();
            entity.Property(u => u.Email).IsRequired().HasMaxLength(320);
            entity.Property(u => u.Name).HasMaxLength(255);
        });
    }
}
`,
		"appsettings.json": `{
  "Logging": {
    "LogLevel": {
      "Default": "Information",
      "Microsoft.AspNetCore": "Warning"
    }
  },
  "AllowedHosts": "*",
  "ConnectionStrings": {
    "Default": "Host=localhost;Database=app;Username=postgres;Password="
  }
}
`,
		"appsettings.Development.json": `{
  "Logging": {
    "LogLevel": {
      "Default": "Debug",
      "Microsoft.AspNetCore": "Information"
    }
  },
  "ConnectionStrings": {
    "Default": "Host=localhost;Database=app_dev;Username=postgres;Password="
  }
}
`,
		"Properties/launchSettings.json": `{
  "profiles": {
    "App": {
      "commandName": "Project",
      "launchBrowser": false,
      "applicationUrl": "http://localhost:5080",
      "environmentVariables": {
        "ASPNETCORE_ENVIRONMENT": "Development"
      }
    }
  }
}
`,
		"Dockerfile": `# Multi-stage .NET image. Build stage uses the SDK; runtime
# stage drops the SDK to ship the smaller aspnet base image.
FROM mcr.microsoft.com/dotnet/sdk:9.0 AS build
WORKDIR /src
COPY App.csproj ./
RUN dotnet restore App.csproj
COPY . .
RUN dotnet publish App.csproj -c Release -o /app/publish /p:UseAppHost=false

FROM mcr.microsoft.com/dotnet/aspnet:9.0 AS runtime
WORKDIR /app
ENV ASPNETCORE_URLS=http://+:8080 ASPNETCORE_ENVIRONMENT=Production
COPY --from=build /app/publish ./
EXPOSE 8080
ENTRYPOINT ["dotnet", "App.dll"]
`,
		".dockerignore": `bin/
obj/
.vs/
.vscode/
.idea/
.git
.gitignore
*.user
*.suo
Properties/launchSettings.json
appsettings.Development.json
`,
		".gitignore": `bin/
obj/
out/
publish/
.vs/
.vscode/
.idea/
*.user
*.suo
*.userosscache
*.sln.docstates
*.swp
.DS_Store
*.log
`,
	}
	contract := `ASP.NET Core scaffold: .NET 9, EF Core + Npgsql, minimal API.

Already provisioned:
- /App.csproj                          → Web SDK, EF Core Design + Npgsql + Swashbuckle
- /Program.cs                          → builder, DbContext, swagger (dev), /health, /users CRUD
- /Models/User.cs                      → POCO
- /Data/AppDbContext.cs                → DbContext + indexes
- /appsettings.json                    → connection string default
- /appsettings.Development.json        → dev overrides
- /Properties/launchSettings.json      → dotnet run profile
- /Dockerfile                          → sdk:9.0 build → aspnet:9.0 runtime

Contract for the Coder:
1. Run locally: dotnet run
2. Database: set DATABASE_URL (postgres:// URL) in production; the
   resolver in Program.cs translates it to an Npgsql connection
   string. Local dev uses the ConnectionStrings:Default from
   appsettings.Development.json.
3. First migration: dotnet ef migrations add Init && dotnet ef
   database update. EF Core Design is already referenced in the
   csproj.
4. Add new resources by introducing a DbSet on AppDbContext, a POCO
   under Models/, and a MapGroup() block in Program.cs.
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
