file_path = 'views/layout.html'
content = File.read(file_path)

new_content = content.gsub(/(\?v=)(\d+)/) do |match|
  prefix, version_number = $1, $2.to_i
  "#{prefix}#{version_number + 1}"
end

File.write(file_path, new_content)
puts "Updated version numbers in #{file_path}"
