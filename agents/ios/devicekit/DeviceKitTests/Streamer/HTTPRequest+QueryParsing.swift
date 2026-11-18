import FlyingFox

extension HTTPRequest {
    func queryInt(name: String, default defaultValue: Int, min minValue: Int, max maxValue: Int) -> Int {
        guard let param = query.first(where: { $0.name == name }),
              let intValue = Int(param.value) else {
            return defaultValue
        }
        return Swift.max(minValue, Swift.min(maxValue, intValue))
    }
}
