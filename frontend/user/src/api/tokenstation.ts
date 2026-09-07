import { userApi } from './client'

export const tokenStationAPI = {
  catalog: () => userApi.get('/public/token-stations'),
}
