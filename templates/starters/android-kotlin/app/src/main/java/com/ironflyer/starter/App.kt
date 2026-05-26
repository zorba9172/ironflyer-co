package com.ironflyer.starter

import androidx.compose.runtime.Composable
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController

@Composable
fun App() {
    val navController = rememberNavController()
    NavHost(navController = navController, startDestination = "home") {
        composable("home") {
            HomeScreen(onOpenDashboard = { navController.navigate("dashboard") })
        }
        composable("dashboard") {
            DashboardScreen(onBack = { navController.popBackStack() })
        }
    }
}
