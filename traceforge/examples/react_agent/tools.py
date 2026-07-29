# SPDX-License-Identifier: MIT
"""Mock tools for TraceForge ReAct demo agent."""


def get_weather(city: str) -> str:
    data = {
        "london": "15°C, partly cloudy",
        "berlin": "22°C, sunny",
        "tokyo": "28°C, humid",
    }
    return data.get(city.lower(), f"Weather data unavailable for {city}")


def convert_currency(amount: float, from_currency: str, to_currency: str) -> str:
    rates = {"USD_EUR": 0.92, "EUR_GBP": 0.84, "USD_GBP": 0.77}
    key = f"{from_currency.upper()}_{to_currency.upper()}"
    rate = rates.get(key)
    if rate is None:
        return f"No rate for {from_currency}→{to_currency}"
    return f"{amount * rate:.2f} {to_currency.upper()}"


def summarize(text: str) -> str:
    words = text.split()
    return " ".join(words[:30]) + ("..." if len(words) > 30 else "")


def finish(answer: str) -> str:
    return answer


TOOLS = {
    "get_weather": get_weather,
    "convert_currency": convert_currency,
    "summarize": summarize,
    "finish": finish,
}
