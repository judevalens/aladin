package com.jvp.aladin_compose.ui_lib

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.hoverable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsHoveredAsState
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.dp

data class AladinInteractionState(
    val source: MutableInteractionSource,
    val hovered: Boolean,
    val pressed: Boolean,
    val enabled: Boolean,
    val selected: Boolean,
)

data class AladinInteractionColors(
    val rest: Color = Color.Transparent,
    val hovered: Color = AladinColor.ControlHover,
    val pressed: Color = AladinColor.ControlPressed,
    val selected: Color = AladinColor.RowSelected,
    val selectedHovered: Color = AladinColor.ControlPressed,
    val disabled: Color = Color.Transparent,
) {
  fun colorFor(state: AladinInteractionState): Color {
    return when {
      !state.enabled -> disabled
      state.pressed -> pressed
      state.selected && state.hovered -> selectedHovered
      state.selected -> selected
      state.hovered -> hovered
      else -> rest
    }
  }
}

object AladinInteractionDefaults {
  val Shape = RoundedCornerShape(6.dp)

  @Composable
  fun colors(
      rest: Color = Color.Transparent,
      hovered: Color = AladinColor.ControlHover,
      pressed: Color = AladinColor.ControlPressed,
      selected: Color = AladinColor.RowSelected,
      selectedHovered: Color = AladinColor.ControlPressed,
      disabled: Color = Color.Transparent,
  ): AladinInteractionColors {
    return AladinInteractionColors(
        rest = rest,
        hovered = hovered,
        pressed = pressed,
        selected = selected,
        selectedHovered = selectedHovered,
        disabled = disabled,
    )
  }
}

@Composable
fun rememberAladinInteractionSource(): MutableInteractionSource {
  return remember { MutableInteractionSource() }
}

@Composable
fun rememberAladinInteractionState(
    source: MutableInteractionSource = rememberAladinInteractionSource(),
    enabled: Boolean = true,
    selected: Boolean = false,
): AladinInteractionState {
  val hovered by source.collectIsHoveredAsState()
  val pressed by source.collectIsPressedAsState()

  return AladinInteractionState(
      source = source,
      hovered = hovered,
      pressed = pressed,
      enabled = enabled,
      selected = selected,
  )
}

@Composable
fun Modifier.aladinInteractiveBackground(
    state: AladinInteractionState,
    colors: AladinInteractionColors = AladinInteractionDefaults.colors(),
    shape: Shape = AladinInteractionDefaults.Shape,
): Modifier {
  return clip(shape).background(colors.colorFor(state), shape)
}

@Composable
fun Modifier.aladinClickable(
    enabled: Boolean = true,
    selected: Boolean = false,
    colors: AladinInteractionColors = AladinInteractionDefaults.colors(),
    shape: Shape = AladinInteractionDefaults.Shape,
    onClick: () -> Unit,
): Modifier {
  val state = rememberAladinInteractionState(enabled = enabled, selected = selected)

  return aladinInteractiveBackground(state = state, colors = colors, shape = shape)
      .hoverable(interactionSource = state.source, enabled = enabled)
      .clickable(
          interactionSource = state.source,
          indication = null,
          enabled = enabled,
          onClick = onClick,
      )
}
