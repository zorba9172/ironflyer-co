package com.ironflyer.starter.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable

private val IronflyerDarkColors = darkColorScheme(
    primary = Violet,
    onPrimary = OnSurface,
    secondary = Coral,
    onSecondary = OnSurface,
    tertiary = Magenta,
    onTertiary = OnSurface,
    background = NearBlack,
    onBackground = OnSurface,
    surface = NearBlack,
    onSurface = OnSurface
)

@Composable
fun IronflyerTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    // Ironflyer ships dark-first; the system flag is accepted for parity but we
    // keep the dark scheme either way to match the design reference.
    val colors = IronflyerDarkColors

    MaterialTheme(
        colorScheme = colors,
        content = content
    )
}
