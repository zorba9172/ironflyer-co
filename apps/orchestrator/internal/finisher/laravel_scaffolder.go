// LaravelScaffolder — PHP Laravel 11 baseline. Triggers when the
// planner picks Laravel or PHP, or when user stories explicitly name
// Laravel. Emits enough files for `composer install && php artisan
// migrate && php artisan serve` to boot a working API with /health
// and a CRUD resource for users.
//
// We do NOT emit composer.lock — let composer resolve fresh on first
// install. The Dockerfile installs deps inside the build stage.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type LaravelScaffolder struct{}

func (LaravelScaffolder) Name() string { return "php-laravel" }

func (LaravelScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "laravel") || strings.Contains(stack, "php") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "laravel") {
			return true
		}
	}
	return false
}

func (LaravelScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"composer.json": `{
  "name": "ironflyer/app",
  "type": "project",
  "description": "Ironflyer-scaffolded Laravel 11 application.",
  "license": "MIT",
  "require": {
    "php": "^8.2",
    "laravel/framework": "^11.0",
    "laravel/sanctum": "^4.0",
    "laravel/tinker": "^2.9"
  },
  "require-dev": {
    "fakerphp/faker": "^1.23",
    "mockery/mockery": "^1.6",
    "nunomaduro/collision": "^8.0",
    "phpunit/phpunit": "^11.0"
  },
  "autoload": {
    "psr-4": {
      "App\\": "app/",
      "Database\\Factories\\": "database/factories/",
      "Database\\Seeders\\": "database/seeders/"
    }
  },
  "autoload-dev": {
    "psr-4": {
      "Tests\\": "tests/"
    }
  },
  "config": {
    "optimize-autoloader": true,
    "preferred-install": "dist",
    "sort-packages": true,
    "allow-plugins": {
      "pestphp/pest-plugin": true,
      "php-http/discovery": true
    }
  },
  "minimum-stability": "stable",
  "prefer-stable": true
}
`,
		"artisan": `#!/usr/bin/env php
<?php
// Artisan entrypoint. Boots the framework + console kernel then
// dispatches the requested command. Run with: php artisan <cmd>
define('LARAVEL_START', microtime(true));

require __DIR__.'/vendor/autoload.php';

$app = require_once __DIR__.'/bootstrap/app.php';

$status = $app->handleCommand(new Symfony\Component\Console\Input\ArgvInput);

exit($status);
`,
		"bootstrap/app.php": `<?php
// Application bootstrap. Laravel 11 collapses the old kernel files
// into this single configuration block; routes + middleware land
// here, exception handling is registered last.
use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Exceptions;
use Illuminate\Foundation\Configuration\Middleware;

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        web: __DIR__.'/../routes/web.php',
        api: __DIR__.'/../routes/api.php',
        commands: __DIR__.'/../routes/console.php',
        health: '/health',
    )
    ->withMiddleware(function (Middleware $middleware) {
        //
    })
    ->withExceptions(function (Exceptions $exceptions) {
        //
    })->create();
`,
		"public/index.php": `<?php
// Front controller. Every HTTP request lands here; Laravel's
// foundation handles dispatch from this single entry point.
use Illuminate\Foundation\Application;
use Illuminate\Http\Request;

define('LARAVEL_START', microtime(true));

require __DIR__.'/../vendor/autoload.php';

(require_once __DIR__.'/../bootstrap/app.php')
    ->handleRequest(Request::capture());
`,
		"app/Http/Controllers/Controller.php": `<?php
// Base controller. App-wide concerns (authorization helpers,
// shared traits) live here so feature controllers stay focused.
namespace App\Http\Controllers;

abstract class Controller
{
}
`,
		"app/Http/Controllers/UserController.php": `<?php
// Example resource controller. All seven Laravel resource actions
// are stubbed so apiResource('/users', UserController::class)
// wires up cleanly without further code.
namespace App\Http\Controllers;

use App\Models\User;
use Illuminate\Http\Request;
use Illuminate\Http\JsonResponse;

class UserController extends Controller
{
    public function index(): JsonResponse
    {
        return response()->json(User::all());
    }

    public function create()
    {
        // API-only scaffold: no create form rendered.
        return response()->noContent();
    }

    public function store(Request $request): JsonResponse
    {
        $data = $request->validate([
            'email' => 'required|email|unique:users,email',
            'name'  => 'nullable|string|max:255',
        ]);
        $user = User::create($data);
        return response()->json($user, 201);
    }

    public function show(User $user): JsonResponse
    {
        return response()->json($user);
    }

    public function edit(User $user)
    {
        // API-only scaffold: no edit form rendered.
        return response()->noContent();
    }

    public function update(Request $request, User $user): JsonResponse
    {
        $data = $request->validate([
            'email' => 'sometimes|email|unique:users,email,'.$user->id,
            'name'  => 'sometimes|string|max:255',
        ]);
        $user->update($data);
        return response()->json($user);
    }

    public function destroy(User $user): JsonResponse
    {
        $user->delete();
        return response()->json(null, 204);
    }
}
`,
		"app/Http/Controllers/HealthController.php": `<?php
// Liveness + readiness endpoint. The framework's bootstrap also
// exposes /health (see bootstrap/app.php), but this controller
// gives the Coder a place to hang real dependency checks.
namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;

class HealthController extends Controller
{
    public function show(): JsonResponse
    {
        return response()->json(['status' => 'ok']);
    }
}
`,
		"app/Models/User.php": `<?php
// User model. Inherits Authenticatable so Sanctum + the standard
// auth guards work out of the box. Mass-assignable fields are
// constrained so request payloads cannot set unintended columns.
namespace App\Models;

use Illuminate\Foundation\Auth\User as Authenticatable;
use Illuminate\Notifications\Notifiable;
use Laravel\Sanctum\HasApiTokens;

class User extends Authenticatable
{
    use HasApiTokens, Notifiable;

    protected $fillable = [
        'name',
        'email',
        'password',
    ];

    protected $hidden = [
        'password',
        'remember_token',
    ];

    protected function casts(): array
    {
        return [
            'email_verified_at' => 'datetime',
            'password' => 'hashed',
        ];
    }
}
`,
		"routes/api.php": `<?php
// API route table. Mounted under /api by withRouting() in
// bootstrap/app.php. Keep this file focused on JSON endpoints —
// HTML lives in routes/web.php.
use App\Http\Controllers\HealthController;
use App\Http\Controllers\UserController;
use Illuminate\Support\Facades\Route;

Route::get('/health', [HealthController::class, 'show']);
Route::apiResource('/users', UserController::class);
`,
		"routes/web.php": `<?php
// Web route table. The scaffold ships a single index route so
// the application has a sensible response at /.
use Illuminate\Support\Facades\Route;

Route::get('/', function () {
    return response()->json(['app' => 'ironflyer', 'docs' => '/api']);
});
`,
		"routes/console.php": `<?php
// Console command registration. Custom artisan commands land
// here via Artisan::command(...).
`,
		"database/migrations/2026_05_24_000000_create_users_table.php": `<?php
// Users table migration. Run via: php artisan migrate
use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration {
    public function up(): void
    {
        Schema::create('users', function (Blueprint $table) {
            $table->id();
            $table->string('name')->nullable();
            $table->string('email')->unique();
            $table->timestamp('email_verified_at')->nullable();
            $table->string('password')->nullable();
            $table->rememberToken();
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('users');
    }
};
`,
		"config/database.php": `<?php
// Database config. Connection defaults are driven by DB_* env
// vars; Laravel will also parse DATABASE_URL transparently when
// it is set (see Illuminate\Support\ConfigurationUrlParser).
return [
    'default' => env('DB_CONNECTION', 'pgsql'),

    'connections' => [
        'pgsql' => [
            'driver'   => 'pgsql',
            'url'      => env('DATABASE_URL'),
            'host'     => env('DB_HOST', '127.0.0.1'),
            'port'     => env('DB_PORT', '5432'),
            'database' => env('DB_DATABASE', 'forge'),
            'username' => env('DB_USERNAME', 'forge'),
            'password' => env('DB_PASSWORD', ''),
            'charset'  => 'utf8',
            'prefix'   => '',
            'prefix_indexes' => true,
            'search_path' => 'public',
            'sslmode'  => env('DB_SSLMODE', 'prefer'),
        ],
    ],

    'migrations' => 'migrations',
];
`,
		"config/app.php": `<?php
// App config. Pin URL + key from env so the same image runs in
// every environment without code changes.
return [
    'name'   => env('APP_NAME', 'Ironflyer'),
    'env'    => env('APP_ENV', 'production'),
    'debug'  => (bool) env('APP_DEBUG', false),
    'url'    => env('APP_URL', 'http://localhost'),
    'timezone' => 'UTC',
    'locale'   => 'en',
    'fallback_locale' => 'en',
    'key' => env('APP_KEY'),
    'cipher' => 'AES-256-CBC',
];
`,
		".env.example": `# Copy to .env and fill in. APP_KEY is generated by:
#   php artisan key:generate
APP_NAME=Ironflyer
APP_ENV=production
APP_DEBUG=false
APP_KEY=
APP_URL=http://localhost

# Either set DATABASE_URL OR the discrete DB_* fields below.
DATABASE_URL=

DB_CONNECTION=pgsql
DB_HOST=127.0.0.1
DB_PORT=5432
DB_DATABASE=app
DB_USERNAME=postgres
DB_PASSWORD=
`,
		"Dockerfile": `# Multi-stage PHP image. Build stage runs composer install with
# dev deps off; runtime stage layers the vendor tree on top of
# php:8.3-fpm and exposes 9000 for the upstream web server.
FROM composer:2 AS deps
WORKDIR /app
COPY composer.json ./
RUN composer install --no-dev --no-interaction --prefer-dist --no-progress --no-scripts

FROM php:8.3-fpm AS runtime
WORKDIR /var/www/html
RUN apt-get update && \
    apt-get install -y --no-install-recommends libpq-dev libzip-dev zip unzip && \
    docker-php-ext-install pdo pdo_pgsql pgsql zip opcache && \
    rm -rf /var/lib/apt/lists/*
COPY --from=deps /app/vendor ./vendor
COPY . .
RUN chown -R www-data:www-data storage bootstrap/cache 2>/dev/null || true
EXPOSE 9000
CMD ["php-fpm"]
`,
		".dockerignore": `.git
.gitignore
.env
.env.*
!.env.example
node_modules
vendor
storage/logs/*
storage/framework/cache/*
storage/framework/sessions/*
storage/framework/views/*
tests
`,
		".gitignore": `/vendor
/node_modules
/public/build
/public/hot
/public/storage
/storage/*.key
/storage/pail
/.env
/.env.backup
/.env.production
.phpunit.result.cache
.phpunit.cache
Homestead.json
Homestead.yaml
auth.json
npm-debug.log
yarn-error.log
/.fleet
/.idea
/.vscode
`,
	}
	contract := `Laravel scaffold: PHP 8.2+, Laravel 11.x, PostgreSQL.

Already provisioned:
- /composer.json                                                 → Laravel 11 + Sanctum + Tinker (composer.lock intentionally absent)
- /artisan, /bootstrap/app.php, /public/index.php                → app bootstrap + front controller
- /app/Http/Controllers/UserController.php                       → all seven resource actions
- /app/Http/Controllers/HealthController.php                     → GET /health
- /app/Models/User.php                                           → Authenticatable + Sanctum
- /routes/api.php                                                → apiResource('/users') + health
- /routes/web.php, /routes/console.php                           → bare web + console route tables
- /database/migrations/2026_05_24_000000_create_users_table.php  → users table
- /config/database.php, /config/app.php                          → env-driven config
- /.env.example                                                  → DATABASE_URL + DB_* placeholders
- /Dockerfile                                                    → composer:2 build → php:8.3-fpm runtime

Contract for the Coder:
1. First boot: composer install && php artisan key:generate && php
   artisan migrate && php artisan serve
2. DATABASE_URL is parsed transparently by Laravel's config layer;
   the discrete DB_* fields are a fallback for local dev.
3. Add new resources with: php artisan make:model Foo -mcr — then
   register the route in /routes/api.php.
4. Sanctum is wired but not enforced. Add the auth:sanctum middleware
   to protected routes once your auth flow is ready.
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
