// @generated from OpenAPI spec — do not edit directly
import Foundation

/// Builds a URL query string from an array of URLQueryItem.
func queryString(_ items: [URLQueryItem]) -> String {
    guard !items.isEmpty else { return "" }
    var components = URLComponents()
    components.queryItems = items
    return "?" + (components.query ?? "")
}