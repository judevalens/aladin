package com.jvp.aladin_compose.service

import kotlin.time.Clock
import kotlin.time.Instant
import kotlinx.datetime.DateTimeUnit
import kotlinx.datetime.LocalDateTime
import kotlinx.datetime.TimeZone
import kotlinx.datetime.until
import kotlinx.datetime.toLocalDateTime

private val MonthNames =
    listOf(
        "Jan",
        "Feb",
        "Mar",
        "Apr",
        "May",
        "Jun",
        "Jul",
        "Aug",
        "Sep",
        "Oct",
        "Nov",
        "Dec",
    )

fun formatUpdatedTimestamp(raw: String): String {
    return formatUpdatedTimestamp(raw = raw, now = Clock.System.now())
}

fun formatUpdatedTimestamp(
    raw: String,
    now: Instant,
): String {
    val timestamp = raw.trim()
    if (timestamp.isEmpty()) return raw

    return try {
        val instant = Instant.parse(timestamp)
        val timeZone = TimeZone.currentSystemDefault()
        val value = instant.toLocalDateTime(timeZone)
        val current = now.toLocalDateTime(timeZone)

        if (value.date == current.date) {
            val minutesAgo = instant.until(now, DateTimeUnit.MINUTE).coerceAtLeast(0)
            when {
                minutesAgo < 1 -> "1 min ago"
                minutesAgo < 60 -> "$minutesAgo min ago"
                else -> "${minutesAgo / 60}h ago"
            }
        } else {
            buildAbsoluteTimestamp(value = value, currentYear = current.year)
        }
    } catch (_: Throwable) {
        raw
    }
}

private fun buildAbsoluteTimestamp(
    value: LocalDateTime,
    currentYear: Int,
): String {
    val month = MonthNames.getOrElse(value.month.ordinal) { "Jan" }
    val day = value.day
    val hours24 = value.hour
    val minutes = value.minute.toString().padStart(2, '0')
    val meridiem = if (hours24 >= 12) "PM" else "AM"
    val hours12 =
        when {
            hours24 == 0 -> 12
            hours24 > 12 -> hours24 - 12
            else -> hours24
        }
    val dateLabel =
        if (value.year == currentYear) {
            "$month $day"
        } else {
            "$month $day, ${value.year}"
        }

    return "$dateLabel at $hours12:$minutes $meridiem"
}
