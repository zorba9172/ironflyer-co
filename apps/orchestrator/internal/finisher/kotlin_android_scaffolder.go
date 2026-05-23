// KotlinAndroidScaffolder — native Android skeleton in Kotlin with
// Jetpack Compose, navigation, Retrofit, and kotlinx.serialization.
// Targets compile SDK 35 / min SDK 26.
//
// Triggers when the spec mentions Android + native, or stories ask for
// an Android app / Kotlin codebase.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type KotlinAndroidScaffolder struct{}

func (KotlinAndroidScaffolder) Name() string { return "kotlin-android" }

func (KotlinAndroidScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "android") && strings.Contains(stack, "native") {
		return true
	}
	if strings.Contains(stack, "kotlin") && strings.Contains(stack, "android") {
		return true
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "android app") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "android app") || strings.Contains(body, "kotlin") {
			return true
		}
	}
	return false
}

func (KotlinAndroidScaffolder) Scaffold(_ context.Context, _ *domain.Project) (DomainScaffold, error) {
	files := map[string]string{
		"app/build.gradle.kts": `// Application module. Keep compile/target SDK in lockstep — Google's
// review tooling rejects mismatches. Compose BOM pins consistent
// versions of every androidx.compose.* artifact.
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.serialization")
}

android {
    namespace = "com.example.app"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.example.app"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"

        // BASE_URL is exposed via BuildConfig so Retrofit can read it
        // without ever hardcoding a host into Kotlin source.
        val baseUrl = (project.findProperty("BASE_URL") as String?) ?: "https://api.example.com"
        buildConfigField("String", "BASE_URL", "\"" + baseUrl + "\"")

        ndk {
            abiFilters += listOf("arm64-v8a", "x86_64")
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.14"
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.09.02")
    implementation(composeBom)
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.6")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.6")
    implementation("androidx.navigation:navigation-compose:2.8.1")

    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")

    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.jakewharton.retrofit:retrofit2-kotlinx-serialization-converter:1.0.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.squareup.okhttp3:logging-interceptor:4.12.0")
}
`,
		"app/src/main/AndroidManifest.xml": `<?xml version="1.0" encoding="utf-8"?>
<!--
  Minimal manifest. INTERNET is the only permission baked in; add
  CAMERA / LOCATION / etc. on-demand from the screen that needs them
  so the Play Console doesn't flag over-broad permissions.
-->
<manifest xmlns:android="http://schemas.android.com/apk/res/android">

    <uses-permission android:name="android.permission.INTERNET" />

    <application
        android:allowBackup="true"
        android:label="Ironflyer Android"
        android:supportsRtl="true"
        android:theme="@style/Theme.Material3.DayNight.NoActionBar">

        <activity
            android:name=".MainActivity"
            android:exported="true"
            android:launchMode="singleTop">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`,
		"app/src/main/java/com/example/app/MainActivity.kt": `// Single-activity host. All UI is Compose — this file only sets the
// content surface and hands control to AppNavHost.
package com.example.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.example.app.ui.AppNavHost

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                Surface(modifier = Modifier) {
                    AppNavHost()
                }
            }
        }
    }
}
`,
		"app/src/main/java/com/example/app/ui/AppNavHost.kt": `// Top-level navigation. Two routes: "home" lists items, "detail/{id}"
// renders one. Add new destinations by appending a composable(...) block.
package com.example.app.ui

import androidx.compose.runtime.Composable
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument

@Composable
fun AppNavHost() {
    val nav = rememberNavController()
    NavHost(navController = nav, startDestination = "home") {
        composable("home") {
            HomeScreen(onItemClick = { id -> nav.navigate("detail/" + id) })
        }
        composable(
            route = "detail/{id}",
            arguments = listOf(navArgument("id") { type = NavType.StringType }),
        ) { entry ->
            val id = entry.arguments?.getString("id") ?: ""
            DetailScreen(id = id, onBack = { nav.popBackStack() })
        }
    }
}
`,
		"app/src/main/java/com/example/app/ui/HomeScreen.kt": `// Home: Material 3 Scaffold with a list of items and a floating
// add button. Replace the hardcoded items with a ViewModel-backed
// flow once you have data.
package com.example.app.ui

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(onItemClick: (String) -> Unit) {
    var items by remember { mutableStateOf(listOf("alpha", "beta", "gamma")) }
    Scaffold(
        topBar = { TopAppBar(title = { Text("Ironflyer Android") }) },
        floatingActionButton = {
            FloatingActionButton(onClick = {
                items = items + ("item-" + (items.size + 1))
            }) {
                Icon(Icons.Filled.Add, contentDescription = "Add")
            }
        },
    ) { padding ->
        LazyColumn(modifier = Modifier.fillMaxSize().padding(padding)) {
            items(items) { name ->
                ListItem(
                    headlineContent = { Text(name) },
                    modifier = Modifier.padding(8.dp),
                )
            }
        }
    }
}
`,
		"app/src/main/java/com/example/app/ui/DetailScreen.kt": `// Detail screen — placeholder. Replace the body with a real read
// once you have a repository / ViewModel layer.
package com.example.app.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DetailScreen(id: String, onBack: () -> Unit) {
    Scaffold(topBar = { TopAppBar(title = { Text("Detail") }) }) { padding ->
        Column(modifier = Modifier.fillMaxSize().padding(padding).padding(16.dp)) {
            Text("Item id: " + id)
            Button(onClick = onBack) { Text("Back") }
        }
    }
}
`,
		"app/src/main/java/com/example/app/network/Api.kt": `// Retrofit interface. BASE_URL is read from BuildConfig (which is set
// by gradle.properties / local.properties at build time) so the URL is
// never hardcoded in Kotlin source.
package com.example.app.network

import com.example.app.BuildConfig
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import retrofit2.Retrofit
import retrofit2.http.GET
import retrofit2.http.Path

@Serializable
data class UserDto(val id: String, val name: String, val email: String)

interface Api {
    @GET("api/users")
    suspend fun listUsers(): List<UserDto>

    @GET("api/users/{id}")
    suspend fun getUser(@Path("id") id: String): UserDto
}

object ApiClient {
    private val json = Json { ignoreUnknownKeys = true; isLenient = true }

    val api: Api by lazy {
        Retrofit.Builder()
            .baseUrl(BuildConfig.BASE_URL.trimEnd('/') + "/")
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(Api::class.java)
    }
}
`,
		"build.gradle.kts": `// Root build script. Plugin versions live here so every module
// inherits the same toolchain. Bump the Kotlin + AGP versions together
// — they have a known compatibility matrix.
plugins {
    id("com.android.application") version "8.6.1" apply false
    id("org.jetbrains.kotlin.android") version "1.9.25" apply false
    id("org.jetbrains.kotlin.plugin.serialization") version "1.9.25" apply false
}
`,
		"settings.gradle.kts": `// Module roster. Add new modules here as the app grows
// (e.g. :core, :feature-checkout, :data).
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "ironflyer-android"
include(":app")
`,
		"gradle.properties": `# Stable Gradle defaults for Android + Kotlin. Bump the JVM heap if
# you start hitting OOM on big modules; 2g is enough for the scaffold.
org.gradle.jvmargs=-Xmx2048m -Dfile.encoding=UTF-8
org.gradle.parallel=true
org.gradle.caching=true
android.useAndroidX=true
android.nonTransitiveRClass=true
kotlin.code.style=official

# Override at the gradle command line or in local.properties for a real
# backend, e.g. BASE_URL=https://api.staging.example.com
BASE_URL=https://api.example.com
`,
		".gitignore": `# Gradle
.gradle/
build/
!gradle/wrapper/gradle-wrapper.jar

# Android Studio / IntelliJ
.idea/
*.iml
local.properties
captures/
.cxx/

# OS
.DS_Store
Thumbs.db

# Keystores — never commit signing material
*.jks
*.keystore
`,
	}
	contract := `Kotlin Android scaffold: Jetpack Compose + Material 3 + Navigation + Retrofit.

Already provisioned:
- /app/build.gradle.kts                                    → compileSdk 35, Compose BOM, Retrofit, kotlinx.serialization
- /app/src/main/AndroidManifest.xml                        → MainActivity declaration + INTERNET permission
- /app/src/main/java/com/example/app/MainActivity.kt       → ComponentActivity, setContent → AppNavHost
- /app/src/main/java/com/example/app/ui/AppNavHost.kt      → NavHost with home + detail/{id}
- /app/src/main/java/com/example/app/ui/HomeScreen.kt      → Material 3 Scaffold, list + FAB add
- /app/src/main/java/com/example/app/ui/DetailScreen.kt    → placeholder detail view
- /app/src/main/java/com/example/app/network/Api.kt        → Retrofit interface reading BuildConfig.BASE_URL
- /build.gradle.kts /settings.gradle.kts /gradle.properties → root build config

Contract for the Coder:
1. Install on a connected device with: ./gradlew :app:installDebug
2. BASE_URL is REQUIRED — set it in local.properties (BASE_URL=https://...)
   or via -PBASE_URL=... on the gradle command line. It is exposed to
   Kotlin source via BuildConfig.BASE_URL.
3. Target ABIs: arm64-v8a (real phones) + x86_64 (emulator).
4. Add new screens under com.example.app.ui and register them in AppNavHost.
5. Add new endpoints to Api.kt; keep DTOs @Serializable.
6. Do NOT commit local.properties or *.keystore — they are gitignored.
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
