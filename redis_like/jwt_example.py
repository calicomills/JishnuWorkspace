from flask import Flask, render_template, request, jsonify
from flask_jwt_extended import JWTManager, jwt_required, create_access_token, get_jwt_identity

app = Flask(__name__)
app.secret_key = 'your_secret_key'  # Replace with a strong secret key

# Configure JWT
app.config['JWT_SECRET_KEY'] = 'jwt_secret_key'  # Replace with a strong JWT secret key
jwt = JWTManager(app)

# Example user data
users = {'user_id': {'password': 'password'}}  # Replace with actual user data

@jwt.user_loader_callback_loader
def user_loader_callback(identity):
    user = users.get(identity)
    return {'username': identity, 'id': identity} if user else None

@app.route('/login', methods=['POST'])
def login():
    data = request.get_json()
    username = data.get('username')
    password = data.get('password')

    # Authenticate user (replace with your actual authentication logic)
    if username in users and users[username]['password'] == password:
        access_token = create_access_token(identity=username)
        return jsonify(access_token=access_token), 200
    else:
        return jsonify(message='Invalid credentials'), 401

@app.route('/protected', methods=['GET'])
@jwt_required()
def protected():
    current_user = get_jwt_identity()
    return jsonify(message=f'Hello, {current_user}! You have access to this protected resource.')

if __name__ == '__main__':
    app.run(debug=True)
